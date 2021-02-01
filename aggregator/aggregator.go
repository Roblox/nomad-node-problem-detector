package aggregator

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/hashicorp/nomad/api"
	types "github.com/nomad-node-problem-detector/types"
)

func Aggregate() {
	client, err := getNomadClient()
	if err != nil {
		log.Fatal(err)
	}

	nodeHandle := client.Nodes()

	queryOptions := &api.QueryOptions{AllowStale: true}

	for {
		log.Info("Collect and aggregate nodes health")
		nodes, _, err := nodeHandle.List(queryOptions)
		if err != nil {
			errMsg := fmt.Sprintf("Error in listing nomad nodes: %v\n", err)
			log.Warning(errMsg)
			time.Sleep(15 * time.Second)
			continue
		}

		for _, node := range nodes {
			npdServer := fmt.Sprintf("http://%s:8083", node.Address)

			npdActive, err := isNpdServerActive(npdServer)
			if err != nil {
				errMsg := fmt.Sprintf("Node %s is unreachable, marking it as ineligible.", node.Address)
				log.Warning(errMsg)
				toggleNodeEligibility(nodeHandle, node.ID, node.Address, false)
				continue
			}

			if !npdActive {
				errMsg := fmt.Sprintf("Node %s is unhealthy, marking it as ineligible.", node.Address)
				log.Warning(errMsg)
				toggleNodeEligibility(nodeHandle, node.ID, node.Address, false)
				continue
			}

			url := npdServer + "/v1/nodehealth/"
			req, err := http.NewRequest("POST", url, nil)
			if err != nil {
				errMsg := fmt.Sprintf("Error in building /v1/nodehealth/ HTTP request, skipping node %s\n", node.Address)
				log.Warning(errMsg)
				continue
			}

			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: time.Second * 5}
			resp, err := client.Do(req)
			if err != nil {
				errMsg := fmt.Sprintf("Error in getting /v1/nodehealth/ HTTP response, skipping node %s\n", node.Address)
				log.Warning(errMsg)
				resp.Body.Close()
				continue
			}

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				errMsg := fmt.Sprintf("Error in reading /v1/nodehealth/ HTTP response, skipping node %s\n", node.Address)
				log.Warning(errMsg)
				resp.Body.Close()
				continue
			}

			resp.Body.Close()

			result := []types.HealthCheck{}
			if err := json.Unmarshal(body, &result); err != nil {
				errMsg := fmt.Sprintf("Error in unmarshalling /v1/nodehealth/ HTTP response body, skipping node %s\n", node.Address)
				log.Warning(errMsg)
				continue
			}

			nodeHealthy := true
			toggleOK := true
			for _, res := range result {
				if res.Result != "Healthy" {
					errMsg := fmt.Sprintf("Node %s is not healthy, marking it as ineligible.", node.Address)
					log.Warning(errMsg)
					toggleOK = toggleNodeEligibility(nodeHandle, node.ID, node.Address, false)
					nodeHealthy = false
					break
				}
			}

			if !toggleOK {
				// Skip node
				continue
			}

			if nodeHealthy {
				toggleNodeEligibility(nodeHandle, node.ID, node.Address, true)
			}
		}
		time.Sleep(15 * time.Second)
	}
}

// Toggle Nomad node eligibility, and returns a boolean to indicate if the operation was successful
// or unsuccessful.
func toggleNodeEligibility(nodeHandle *api.Nodes, nodeID, nodeAddress string, eligible bool) bool {
	if _, err := nodeHandle.ToggleEligibility(nodeID, eligible, nil); err != nil {
		errMsg := fmt.Sprintf("Error in toggling node eligibility, skipping node %s\n", nodeAddress)
		log.Warning(errMsg)
		return false
	}
	return true
}

// Check if Nomad node problem detector (nNPD) HTTP server is healthy and active.
func isNpdServerActive(npdServer string) (bool, error) {
	url := npdServer + "/v1/health/"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return false, err
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

// Get Nomad HTTP client.
// This client will be used to list nodes and toggle node eligibility.
func getNomadClient() (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = "http://localhost:4646"
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
