/*
Copyright 2021 Roblox Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0


Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aggregator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/hashicorp/nomad/api"
	types "github.com/nomad-node-problem-detector/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/urfave/cli/v2"
)

var AggregatorCommand = &cli.Command{
	Name:  "aggregator",
	Usage: "Run npd in aggregator mode",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "aggregation-cycle-time",
			Aliases: []string{"t"},
			Value:   "15s",
			Usage:   "Time (in seconds) to wait between each aggregation cycle",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug logging.",
		},
		&cli.StringFlag{
			Name:    "detector-port",
			Aliases: []string{"p"},
			Value:   ":8083",
			Usage:   "Detector HTTP server port",
		},
		&cli.StringSliceFlag{
			Name:    "detector-datacenter",
			Aliases: []string{"dc"},
			Usage:   "List of datacenters where detector is running.",
		},
		&cli.StringSliceFlag{
			Name:    "enforce-health-check",
			Aliases: []string{"hc"},
			Usage:   "Health checks in this list will be enforced i.e. node will be taken out of the scheduling pool if health-check fails.",
		},
		&cli.StringFlag{
			Name:    "nomad-server",
			Aliases: []string{"s"},
			Value:   "http://localhost:4646",
			Usage:   "HTTP API address of a Nomad server or agent.",
		},
		&cli.StringSliceFlag{
			Name:  "node-attribute",
			Usage: "Aggregator will filter nodes based on these attributes. E.g. if you set os.name=ubuntu, aggregator will only reach out to ubuntu nodes in the cluster.",
		},
		&cli.IntFlag{
			Name:  "threshold-percentage",
			Value: 85,
			Usage: "If the number of eligible nodes goes below the threshold, npd will stop marking nodes as ineligible",
		},
		&cli.IntFlag{
			Name:  "prometheus-server-port",
			Value: 3000,
			Usage: "The port used to expose aggregators metrics in prometheus format",
		},
		&cli.StringFlag{
			Name:  "prometheus-server-addr",
			Value: "0.0.0.0",
			Usage: "The address to bind the aggregator metrics exporter",
		},
	},
	Action: func(c *cli.Context) error {
		return aggregate(c)
	},
}

var (
	pause             bool
	enforceHCMap      map[string]bool
	detectorDCMap     map[string]bool
	nodeAttributesMap map[string]string
)

func aggregate(context *cli.Context) error {
	debug := context.Bool("debug")
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	nomadServer := context.String("nomad-server")
	thresholdPercentage := context.Int("threshold-percentage")
	if thresholdPercentage == 85 {
		log.Warning(fmt.Sprintf("No override set for --threshold-percentage. Running with default value: %d\n", thresholdPercentage))
		log.Warning("Recommended to set an override for --threshold-percentage based on your cluster capacity.")
	}

	client, err := getNomadClient(nomadServer)
	if err != nil {
		return err
	}

	enforceHCList := context.StringSlice("enforce-health-check")
	enforceHCMap = make(map[string]bool)
	for _, hc := range enforceHCList {
		enforceHCMap[hc] = true
	}

	// Create the map of datacenters (DCs) where detector is running.
	detectorDCList := context.StringSlice("detector-datacenter")
	detectorDCMap = make(map[string]bool)
	for _, dc := range detectorDCList {
		detectorDCMap[dc] = true
	}

	// Read the node attributes, and populate the attributes map.
	nodeAttributes := context.StringSlice("node-attribute")
	nodeAttributesMap = make(map[string]string)
	for _, attribute := range nodeAttributes {
		result := strings.Split(attribute, "=")
		if len(result) != 2 {
			return fmt.Errorf("Invalid --node-attribute. Set key=val for valid node attribute.")
		}
		nodeAttributesMap[result[0]] = result[1]
	}

	aggregationCycleTime, err := time.ParseDuration(context.String("aggregation-cycle-time"))
	if err != nil {
		return err
	}

	detectorPort := context.String("detector-port")

	authToken := os.Getenv("DETECTOR_HTTP_TOKEN")

	// Read aggregator DC (Datacenter).
	// $NOMAD_DC along with detector-datacenter list will be used
	// when reaching out to npd detectors.
	datacenter := os.Getenv("NOMAD_DC")
	if datacenter == "" {
		return fmt.Errorf("NOMAD_DC environment variable missing. Datacenter must be set.")
	}
	detectorDCMap[datacenter] = true

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1)
	go flipPause(sigs)

	metricsExporter(context.String("prometheus-server-addr"), context.Int("prometheus-server-port"))

	nodeHandle := client.Nodes()

	queryOptions := &api.QueryOptions{AllowStale: true}

	// Aggregation cycle index
	index := 0

	// map[nodeID][node health check /v1/nodehealth/]
	m := make(map[string][]types.HealthCheck)
	for {
		aggregatorCyclesTotalCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
		if pause {
			// Aggregator is paused. Wait for unpause.
			continue
		}

		log.Info("Collect and aggregate nodes health")
		startTime := time.Now()

		nodes, _, err := nodeHandle.List(queryOptions)
		if err != nil {
			log.Warning(fmt.Sprintf("Error in listing nomad nodes: %v\n", err))
			time.Sleep(aggregationCycleTime)
			continue
		}

		eligibleNodeCount := getEligibleNodeCount(nodes)
		eligibleNodesGauge.With(prometheus.Labels{"dc": datacenter}).Set(float64(eligibleNodeCount))

		totalNodeCount := len(nodes)
		nodesTotalGauge.With(prometheus.Labels{"dc": datacenter}).Set(float64(totalNodeCount))

		log.Info(fmt.Sprintf("Eligible Nodes: %d, Total Nodes: %d", eligibleNodeCount, totalNodeCount))

		var nodeInfo *api.Node
		for _, node := range nodes {
			nodeInfo, _, err = nodeHandle.Info(node.ID, queryOptions)
			if err != nil {
				log.Warning(fmt.Sprintf("Error in getting node info: %v. Skipping node: %s\n", err, node.Address))
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				continue
			}

			skipNode := false
			for key, val := range nodeAttributesMap {
				res, ok := nodeInfo.Attributes[key]
				if !ok {
					if debug {
						log.Debug(fmt.Sprintf("Node %s: node attribute: %s doesn't exist, skipping node.", node.Address, key))
					}
					skipNode = true
					break
				}

				if res != val {
					if debug {
						log.Debug(fmt.Sprintf("Node %s: node attribute: %s doesn't match. Expected: %s, actual: %s. Skipping node...", node.Address, key, val, res))
					}
					skipNode = true
					break
				}

			}

			if _, ok := detectorDCMap[nodeInfo.Datacenter]; !ok {
				skipNode = true
			}

			// If node attribute e.g. os.name=ubuntu is missing or not matching in the node info
			// OR node is not in a DC where detector is running, Skip this node, and move onto next one.
			if skipNode {
				nodeHandleSkipCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				continue
			}

			npdServer := fmt.Sprintf("http://%s%s", node.Address, detectorPort)

			npdActive, err := isNpdServerActive(npdServer, authToken)
			if err != nil {
				log.Warning(fmt.Sprintf("NNPD detector server is not active, maybe node %s was ineligible when npd was deployed, skipping node.", node.Address))
				if debug {
					log.Debug(fmt.Sprintf("Error: %v\n", err))
				}
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				continue
			}

			if !npdActive {
				log.Warning(fmt.Sprintf("Node problem detector /v1/health on node %s is unhealthy, skipping node.", node.Address))
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				continue
			}

			url := npdServer + "/v1/nodehealth/"
			req, err := http.NewRequest("POST", url, nil)
			if err != nil {
				log.Warning(fmt.Sprintf("Error in building /v1/nodehealth/ HTTP request: %v, skipping node %s\n", err, node.Address))
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				continue
			}

			if authToken != "" {
				base64EncodedToken := base64.StdEncoding.EncodeToString([]byte(authToken))
				req.Header.Set("Authorization", "Basic "+base64EncodedToken)
			}

			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: time.Second * 5}
			resp, err := client.Do(req)
			if err != nil {
				log.Warning(fmt.Sprintf("Error in getting /v1/nodehealth/ HTTP response: %v, skipping node %s\n", err, node.Address))
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				resp.Body.Close()
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Warning(fmt.Sprintf("Error in reading /v1/nodehealth/ HTTP response: %v, skipping node %s\n", err, node.Address))
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				resp.Body.Close()
				continue
			}

			resp.Body.Close()

			current := []types.HealthCheck{}
			if err := json.Unmarshal(body, &current); err != nil {
				log.Warning(fmt.Sprintf("Error in unmarshalling /v1/nodehealth/ HTTP response body: %v, skipping node %s\n", err, node.Address))
				nodeHandleErrorsCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
				continue
			}

			var nodeHealth []types.HealthCheck
			if m[node.ID] != nil {
				nodeHealth = m[node.ID]
			}

			// previous state map has the health check results from last aggregation cycle.
			// This will make sure we don't toggle/untoggle a node unless there is a state change.
			previous := make(map[string]types.HealthCheck)
			for _, nh := range nodeHealth {
				previous[nh.Type] = nh
			}

			nodeHealthy := true
			stateChanged := false
			toggle := false

			for _, curr := range current {
				// Default CPU, memory and disk checks are represented with
				// boolean (true/false). curr.Result = true for CPUUnderPressure
				// or MemoryUnderPressure or DiskUsageHigh tells that the system
				// is under CPU/memory/disk pressure and should be taken out of
				// eligibility.
				if curr.Result == "Unhealthy" || curr.Result == "true" {
					log.Warning(fmt.Sprintf("Node %s: %s is %s: %s\n", node.Address, curr.Type, curr.Result, curr.Message))
					nodeHealthy = false
					nodeUnhealthyCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
					// Even if one of the health checks are failing, node will not be taken out of the scheduling pool.
					// Unless that health check is part of --enforce-health-check list.
					// Set toggle=true if above is satisfied.
					if _, ok := enforceHCMap[curr.Type]; ok {
						log.Info(fmt.Sprintf("%s is in enforce health check list. Set node %s scheduling eligibility to false\n", curr.Type, node.Address))
						toggle = true
					} else {
						log.Info(fmt.Sprintf("%s is not in enforce health check list. Node %s will be dry-runned and not taken out of scheduling pool\n", curr.Type, node.Address))
					}
				} else {
					nodeHealthyCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
					if debug {
						log.Debug(fmt.Sprintf("Node %s: %s is %s: %s\n", node.Address, curr.Type, curr.Result, curr.Message))
					}
				}

				prev, ok := previous[curr.Type]
				if ok {
					if prev.Result == curr.Result {
						continue
					} else {
						stateChanged = true
						nodeHealthStateChangedCounter.With(prometheus.Labels{"dc": datacenter}).Inc()
					}
				}
			}

			// If toggle is true i.e we want to take the node out of the scheduling pool.
			// We should only take the node out, if the available capacity stays above the threshold (--threshold-percentage)
			// after taking this node out of the scheduling pool.
			aboveThreshold := (float64(eligibleNodeCount)/float64(totalNodeCount))*100 > float64(thresholdPercentage)
			toggle = toggle && aboveThreshold

			// This is the check for first aggregation cycle. No previous state exist at this point.
			if len(previous) == 0 && !nodeHealthy && toggle {
				eligibleNodeCount = toggleNodeEligibility(nodeHandle, node.ID, node.Address, false, eligibleNodeCount)
			}

			// Second aggregation cycle onwards, previous state map will exist.
			if stateChanged {
				if nodeHealthy {
					eligibleNodeCount = toggleNodeEligibility(nodeHandle, node.ID, node.Address, true, eligibleNodeCount)
				} else if toggle {
					eligibleNodeCount = toggleNodeEligibility(nodeHandle, node.ID, node.Address, false, eligibleNodeCount)
				}
			}
			m[node.ID] = current
		}

		endTime := time.Now()
		diff := endTime.Sub(startTime).Seconds()
		log.Info(fmt.Sprintf("Aggregation cycle %d: processing time: %.2f seconds.", index, diff))
		aggregatorProcessingTime.With(prometheus.Labels{"dc": datacenter}).Set(diff)

		index++

		time.Sleep(aggregationCycleTime)
	}
	return nil
}

// getEligibleNodeCount return the count of eligible nodes.
func getEligibleNodeCount(nodes []*api.NodeListStub) int {
	eligibleNodeCount := 0
	for _, node := range nodes {
		if node.SchedulingEligibility == "eligible" {
			eligibleNodeCount++
		}
	}
	return eligibleNodeCount
}

// Toggle Nomad node eligibility.
func toggleNodeEligibility(nodeHandle *api.Nodes, nodeID, nodeAddress string, eligible bool, eligibleNodeCount int) int {
	if _, err := nodeHandle.ToggleEligibility(nodeID, eligible, nil); err != nil {
		log.Warning(fmt.Sprintf("Error in toggling node eligibility, skipping node %s\n", nodeAddress))
		return eligibleNodeCount
	}
	log.Info(fmt.Sprintf("Node %s scheduling eligibility changed to %t\n", nodeAddress, eligible))

	if eligible {
		eligibleNodeCount++
	} else {
		eligibleNodeCount--
	}
	return eligibleNodeCount
}

// Check if Nomad node problem detector (nNPD) HTTP server is healthy and active.
func isNpdServerActive(npdServer, authToken string) (bool, error) {
	url := npdServer + "/v1/health/"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return false, err
	}

	if authToken != "" {
		base64EncodedToken := base64.StdEncoding.EncodeToString([]byte(authToken))
		req.Header.Set("Authorization", "Basic "+base64EncodedToken)
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 5}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, nil
	}
	return true, nil
}

// flipPause pauses and unpauses aggregator based on receiving SIGUSR1 signal.
func flipPause(sigs chan os.Signal) {
	for {
		select {
		case <-sigs:
			pause = !pause
			if pause {
				log.Info("Received signal SIGUSR1, pausing aggregator.")
			} else {
				log.Info("Received signal SIGUSR1, unpausing aggregator.")
			}
		}
	}
}

// Get Nomad HTTP client.
// This client will be used to list nodes and toggle node eligibility.
func getNomadClient(nomadServer string) (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = nomadServer
	cfg.TLSConfig.Insecure = true

	timeout, err := time.ParseDuration("5s")
	if err != nil {
		return nil, err
	}

	cfg.HttpClient = &http.Client{Timeout: timeout}
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return client, nil
}
