package aggregator

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/nomad/api"
	types "github.com/nomad-node-problem-detector/types"
)

func Aggregate() {
	client, err := getNomadClient()
	if err != nil {
		log.Fatalln(err)
	}

	nodeHandle := client.Nodes()

	queryOptions := &api.QueryOptions{AllowStale: true}

	for {
		fmt.Println("Collect and aggregate nodes health")
		nodes, _, err := nodeHandle.List(queryOptions)
		if err != nil {
			log.Fatalln(err)
		}

		for _, node := range nodes {
			url := fmt.Sprintf("http://%s:8083/v1/nodehealth", node.Address)
			req, err := http.NewRequest("POST", url, nil)
			if err != nil {
				log.Fatalln(err)
			}

			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: time.Second * 5}
			resp, err := client.Do(req)
			if err != nil {
				log.Fatalln(err)
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Fatalln(err)
			}

			result := []types.HealthCheck{}

			if err := json.Unmarshal(body, &result); err != nil {
				log.Fatalln(err)
			}

			nodeHealthy := true
			for _, res := range result {
				if res.Result != "Healthy" {
					fmt.Printf("Node %s is not healthy, mark it as ineligible\n", node.Address)
					if _, err := nodeHandle.ToggleEligibility(node.ID, false, nil); err != nil {
						log.Fatalln(err)
					}
					nodeHealthy = false
					break
				}
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
