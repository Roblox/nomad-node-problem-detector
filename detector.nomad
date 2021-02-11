job "detector" {
  datacenters = ["dc1"]

  group "detector-group" {
    network {
      port "http" {
        to = 8083
      }
    }

    task "detector-task" {
      driver = "raw_exec"
      artifact {
        source      = "git::https://github.com/shishir-a412ed/nomad-health-checks.git"
        destination = "local/var/lib/nnpd"
      }

      config {
	command = "npd"
	args    = ["detector"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}
