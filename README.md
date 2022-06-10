# Nomad-node-problem-detector (NNPD)
[![CI Actions Status](https://github.com/Roblox/nomad-node-problem-detector/workflows/CI/badge.svg)](https://github.com/Roblox/nomad-node-problem-detector/actions)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/Roblox/nomad-node-problem-detector/blob/main/LICENSE)
[![Release](https://img.shields.io/badge/version-0.4-blue)](https://github.com/Roblox/nomad-node-problem-detector/releases/tag/v0.4)

"A distributed system is a collection of autonomous compute nodes (sometimes unreliable) that appears to it's users
as a single coherent reliable system"

The goal of Nomad-node-problem-detector (a.k.a NNPD) is to abstract these nodes problems from the user, so that
user experience is more reliable, when using the Nomad orchestration system.

## Motivation

When a user submits a job ( job --> task_groups(N) --> tasks(N) ) each task in the job needs a task driver e.g. `docker`, `java`, `QEMU`, `containerd` etc to execute this task. In the current architecture, if a task driver e.g. `docker` is `Unhealthy` on a Nomad client node and one of the tasks in the job requires `docker` to execute, Nomad scheduler will not schedule this job on that particular Nomad client.

**Question:** What is the definition of a task driver being unhealthy?<br/>
**Answer:** A task driver executes a [`Fingerprint`](https://www.nomadproject.io/docs/internals/plugins/task-drivers#fingerprint-context-context-chan-fingerprint-error) operation every `X` seconds (configurable within the task driver) and reports it's `HealthState` to the Nomad client. Nomad client reports this `HealthState` to the scheduler. Scheduler can then schedule jobs based on the health states of all the task drivers running on each nomad client nodes.

An example fingerprinting operation for [`docker task driver`](https://github.com/hashicorp/nomad/blob/5f968baf0d0b0315ed1b808a4fc43ea2372ee1e9/drivers/docker/fingerprint.go#L80).

However,<br/>
- If I need to add a custom health check in docker task driver, I would have to modify the fingerprinting operation [`here`](https://github.com/hashicorp/nomad/blob/5f968baf0d0b0315ed1b808a4fc43ea2372ee1e9/drivers/docker/fingerprint.go#L80), add this new check and open a PR to [`hashicorp/nomad`](https://github.com/hashicorp/nomad) repo. This custom health check could also be specific to my environment, so adding it to the upstream [`hashicorp/nomad`](https://github.com/hashicorp/nomad) repo might not be possible. NNPD decouples this from the `hashicorp/nomad` codebase, and provides a framework for adding custom health checks easily.<br/>
- Nomad clients could be running under `CPU`, `memory` or `disk` pressure at various times. NNPD constantly monitor the nodes for these situations and take the nodes out of the scheduling pool when they are under `CPU`, `memory` or `disk` pressure. It also put the nodes back into the scheduling pool when the pressure is relieved.<br/>
- The scheduler is only concerned with the task driver health state, when scheduling jobs. However there could be additional problems happening on the node. e.g. ntp service down, kernel issues, corrupted file systems. These can be integrated with NNPD, so nodes can be taken out of the scheduling pool if the node is unhealthy.

In a nutshell, NNPD provides a blackbox (a framework) where we can dump all our node problems, and if a node is running into one of these problems, NNPD will take the node out of the scheduling pool, so no new jobs gets scheduled on this faulty node, until the problem is fixed. In case of a transient issue, if the node recovers, NNPD will also move the node back to the scheduling pool, so new jobs can be scheduled on this node.

**NOTE:** NNPD as the name suggests `Nomad-node-problem-detector` is **only** concerned with the problems happening on the node. Problems external to the node e.g. docker registry down should not be added to NNPD, otherwise it might take all the nodes out of the scheduling pool.

## Architecture

<img src="images/nnpd_arch.png" width="850" height="450" />

NNPD is composed of two main components:

- **Detector:** is responsible for scanning through the node health checks, and exposing the node health at
  `/v1/nodehealth` HTTP endpoint. It also exposes a `/v1/health` endpoint, which tells if the `detector`
  itself is `healthy` or `unhealthy`.

  Detector relies on an external health check repo, which is used for defining the node health checks.<br/>
  A sample health check repo is provided for reference: https://github.com/shishir-a412ed/nomad-health-checks

  **NOTE:** The sample [`health check repo`](https://github.com/shishir-a412ed/nomad-health-checks) **do not**
  contain real health checks, but only provides a reference for defining your own health checks.

- **Aggregator:** is responsible for getting the node health (`/v1/nodehealth`) for each node running `detector`.
  Based on the node health results, aggregator will mark the node as `eligible` or `ineligible` for scheduling.

NNPD is packaged as a single `go` binary, which can be run either in `detector` or `aggregator` mode.

## Building Nomad-node-problem-detector (NNPD)

```
$ git clone git@github.com:Roblox/nomad-node-problem-detector.git
$ cd nomad-node-problem-detector
$ make build (This will build your npd binary)
$ make install (This will install npd binary in your /usr/local/bin)
```

**NOTE:** The binary name is `npd` eventhough the application is called `nnpd`.

## Setup health checks repo

As mentioned in the [Architecture](#Architecture) section, `detector` relies on an external health check repo
for determining the node health (`/v1/nodehealth`). A separate github repository can be defined for your health checks. This [`sample repo`](https://github.com/shishir-a412ed/nomad-health-checks) can be used as a reference.

At the root of the health check repo, a master config ([`config.json`](https://github.com/shishir-a412ed/nomad-health-checks/blob/main/config.json)) will be defined. It has two main fields:

- **type**: Directory name where the actual health check (`health_check`) is located.
- **health_check**: Name of the health check script file.

e.g. In the [`sample config.json`](https://github.com/shishir-a412ed/nomad-health-checks/blob/main/config.json), type `docker` and health_check `docker_health_check.sh` defines that `docker_health_check.sh` will be located under `docker` directory in the nomad health checks repo.

## Deploy

### Prerequisite:

- `npd` should be installed on all `Nomad` client nodes.
- `detector` should be deployed before `aggregator`.

### Deploy detector

`detector` can be deployed either using [`artifactory based job`](https://github.com/Roblox/nomad-node-problem-detector/blob/main/deploy/detector-artifact.nomad) or a [`docker prestart hook based job`](https://github.com/Roblox/nomad-node-problem-detector/blob/main/deploy/detector-docker.nomad)

**NOTE:** You only need to deploy `detector` using one of the modes, **not** both.

In either deployment mode ([`artifactory`](#deploy-detector-artifactory-mode) or [`docker prestart hook`](#deploy-detector-docker-prestart-hook-mode)), `detector` first unpacks the health check repo
onto the Nomad client filesystem under Nomad allocation directory, so that the `detector` can scan (and execute)
these health checks and expose the node health (`/v1/nodehealth`) for the `aggregator`, followed by starting the `detector` daemon.

#### Deploy detector (artifactory mode)

- Modify the artifactory [`source`](https://github.com/Roblox/nomad-node-problem-detector/blob/main/deploy/detector-artifact.nomad#L15) to point to your health check repo. Please check [Setup health check repo](#setup-health-checks-repo) on how to setup your health check repo.
```
$ nomad job plan detector-artifact.nomad
$ nomad job run detector-artifact.nomad
$ nomad job status detector
```

#### Deploy detector (docker prestart hook mode)

[`How to deploy detector using docker prestart hook`](https://github.com/Roblox/nomad-node-problem-detector/wiki/How-to-deploy-detector-using-docker-prestart-hook)

### Deploy aggregator

Official aggregator docker image: `shm32/npd-aggregator:1.1.0`<br/>
You can find the `aggregator` nomad job spec [`here`](https://github.com/Roblox/nomad-node-problem-detector/blob/main/deploy/aggregator.nomad)

```
$ nomad job plan aggregator.nomad
$ nomad job run aggregator.nomad
$ nomad job status aggregator
```

## Rolling upgrades

So, you were able to deploy `detector` and `aggregator` successfully. We have NNPD system up and running.

### Detector upgrade (artifactory mode)

**Question:** [`How do I add a new health check, and do a rolling upgrade on `detector`?`](https://github.com/Roblox/nomad-node-problem-detector/wiki/Detector:-Rolling-upgrade-using-artifactory-mode)

### Detector upgrade (docker prestart hook mode)

- git clone \<your_health_check_repo\>
- Add your new health check in the locally cloned copy.
- Don't forget to update your master config (`config.json`).<br/>
**hint:** Use `npd config generate --root-dir <dir>` to update your master config.
- Follow these [`instructions`](https://github.com/Roblox/nomad-node-problem-detector/wiki/How-to-deploy-detector-using-docker-prestart-hook) to upgrade your `detector` using `docker prestart hook mode`.

## Authentication

You can enable a token based `authentication` for detector HTTP endpoints (`/v1/health/` and `/v1/nodehealth/`) by starting the `detector` with `--auth` flag.

`DETECTOR_HTTP_TOKEN=<your_token>` environment variable **must** be set when deploying `aggregator` and `detector` jobs.<br/>
`aggregator` will use `DETECTOR_HTTP_TOKEN` to set the token in the authorization header when making the HTTP requests.<br/>
`detector` will use `DETECTOR_HTTP_TOKEN` for validating against the incoming token in the authorization header.

```
$ DETECTOR_HTTP_TOKEN=<your_token> npd detector --auth
```

The token is `base64` encoded, so if you are trying things out using `curl`, you need to encode the token first before passing it in the authorization header.

```
$ echo -n <your_token> | base64
$ Note down your base64 encoded token.
$ curl -H "Authorization: Basic <base64_encoded_token>" http://localhost:8083/v1/nodehealth/
```

**NOTE:** In order to keep `NNPD` performant and lightweight, TLS is not support at this point.

## Commands and Flags

**Aggregator** - Run npd in aggregator mode

`npd aggregator --help` for more info.

| Option | Type | Required | Default | Description |
| :---: | :---: | :---: | :---: | :--- |
| **aggregation-cycle-time** | string | no | `15s` | Time (in seconds) to wait between each aggregation cycle. |
| **debug** | bool | no | false | Enable debug logging. |
| **detector-port** | string | no | `:8083` | Detector HTTP server port |
| **detector-datacenter** | []string | no | N/A | List of datacenters where detector is running. If no datacenters are provided, aggregator will only reach out to nodes in `$NOMAD_DC` datacenter. |
| **enforce-health-check** | []string | no | N/A | Health checks in this list will be enforced i.e. node will be taken out of the scheduling pool if health-check fails. |
| **nomad-server** | string | no | `http://localhost:4646` | HTTP API address of a Nomad server or agent. |
| **node-attribute** | []string | no | N/A | Aggregator will filter nodes based on these attributes. E.g. if you set `os.name=ubuntu`, aggregator will only reach out to ubuntu nodes in the cluster. |
| **threshold-percentage** | int | no | `85` | If the number of eligible nodes goes below the threshold, `npd` will stop marking nodes as ineligible. |
| **prometheus-server-port** | int | no | `3000` | The port used to expose aggregator metrics in the prometheus format |
| **prometheus-server-addr** | string | no | `0.0.0.0` | The address to bind the aggregator metrics exporter |

**Detector** - Run nomad node problem detector HTTP server

`npd detector --help` for more info.

| Option | Type | Required | Default | Description |
| :---: | :---: | :---: | :---: | :--- |
| **detector-cycle-time** | string | no | `3s` | Time (in seconds) to wait between each detector cycle. |
| **port** | string | no | `:8083` | Address to listen on for detector HTTP server.<br/> **NOTE:** If your `detector` is listening on a non-default port, don't forget to start your `aggregator` with `--detector-port` flag. This will inform `aggregator` which `detector` port to reach out to. |
| **auth** | bool | no | false | If set to true, `detector` must set `DETECTOR_HTTP_TOKEN=<your_token>` as an environment variable when starting `detector`. |
| **root-dir** | string | no | `/var/lib/nnpd` | Location of health checks. |
| **cpu-limit** | string | no | `85` | CPU threshold in percentage. |
| **memory-limit** | string | no | `80` | Memory threshold in percentage. |
| **disk-limit** | string | no | `90` | Disk threshold in percentage. |

**Config** - Run config and health checks related commands.

`npd config --help` for more info.

There are two subcommands in `npd config` command:

- **npd config generate** - Generates the config.

| Option | Type | Required | Default | Description |
| :---: | :---: | :---: | :---: | :--- |
| **root-dir** | string | no | `pwd - present working directory` | Location of health checks |

- **npd config build** - Copy your health checks into a docker image.

| Option | Type | Required | Default | Description |
| :---: | :---: | :---: | :---: | :--- |
| **image** | string | yes | `N/A` | Fully qualified docker image name |
| **root-dir** | string | no | `pwd - present working directory` | Location of health checks |

## Tests

`vagrant up` will start a local vagrant VM `nnpd`, which has all the dependencies (e.g. nomad, golang) already installed, which are required to run the integration tests.

To run the tests locally in the vagrant VM.

```
$ vagrant up
$ vagrant ssh nnpd
$ sudo make test
```

## Cleanup

```
make clean
```
This will delete your `npd` binary.

```
vagrant destroy
```
This will destroy your vagrant VM.

## License

Copyright 2021 Roblox Corporation

Licensed under the Apache License, Version 2.0 (the "License"). For more information read the [License](LICENSE).
