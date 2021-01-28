package detector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

type HealthCheck struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message string `json:"messgae"`
}

type Config struct {
	Type        string `json:"type"`
	HealthCheck string `json:"health_check"`
}

var (
	m     map[string]*HealthCheck
	mutex = &sync.Mutex{}
)

func StartNpdHttpServer() error {
	fmt.Println("Starting nomad node problem detector...")

	done := make(chan bool, 1)
	go collect(done)
	<-done

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to Nomad node problem detector!\n"))
	})
	http.HandleFunc("/v1/health/", healthCheckHandler)
	http.HandleFunc("/v1/nodehealth/", nodeHealthHandler)

	fmt.Println("nomad node problem detector ready to receive requests.")
	if err := http.ListenAndServe(":8083", nil); err != nil {
		return err
	}
	return nil
}

func readConfig(configPath string, dest interface{}) error {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

func collect(done chan bool) {
	startServer := false
	configPath := "/var/lib/nnpd/config.json"
	dest := []Config{}
	if err := readConfig(configPath, &dest); err != nil {
		log.Fatalln(err)
	}

	for {
		fmt.Println("Collecting node health")
		for _, cfg := range dest {
			fmt.Println(cfg.Type + " " + cfg.HealthCheck)
		}

		if !startServer {
			fmt.Println("Starting nnpd server")
			startServer = true
			done <- startServer
		}
		time.Sleep(3 * time.Second)
	}

}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /v1/health/")
	w.WriteHeader(http.StatusOK)
}

func nodeHealthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /v1/nodehealth/")
	w.WriteHeader(http.StatusOK)
}
