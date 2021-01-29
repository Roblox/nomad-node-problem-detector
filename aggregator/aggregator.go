package aggregator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/nomad/api"
)

func Aggregate() {
	client, err := getNomadClient()
	if err != nil {
		log.Fatalln(err)
	}

	queryOptions := &api.QueryOptions{AllowStale: true}

	for {
		fmt.Println("Collect and aggregate nodes health")
		nodes, _, err := client.Nodes().List(queryOptions)
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

			var out bytes.Buffer
			err = json.Indent(&out, body, "", "    ")
			if err != nil {
				log.Fatalln(err)
			}

			out.WriteTo(os.Stdout)
			fmt.Println()
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
