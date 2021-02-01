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
		fmt.Println("Collect and aggregate nodes health")
		nodes, _, err := nodeHandle.List(queryOptions)
		if err != nil {
			errMsg := fmt.Sprintf("Error in listing nomad nodes: %v\n", err)
			log.Warning(errMsg)
			continue
		}

		for _, node := range nodes {
			url := fmt.Sprintf("http://%s:8083/v1/nodehealth/", node.Address)
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
			toggleEligibilityError := false
			for _, res := range result {
				if res.Result != "Healthy" {
					log.Info(fmt.Sprintf("Node %s is not healthy, marking it as ineligible.", node.Address))
					if _, err := nodeHandle.ToggleEligibility(node.ID, false, nil); err != nil {
						errMsg := fmt.Sprintf("Error in toggling node eligibility, skipping node %s\n", node.Address)
						log.Warning(errMsg)
						toggleEligibilityError = true
						break
					}
					nodeHealthy = false
					break
				}
			}

			if toggleEligibilityError {
				continue
			}

			if nodeHealthy {
				if _, err := nodeHandle.ToggleEligibility(node.ID, true, nil); err != nil {
					log.Fatalln(err)
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

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
