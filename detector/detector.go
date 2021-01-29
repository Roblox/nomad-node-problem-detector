package detector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	types "github.com/nomad-node-problem-detector/types"
)

const NNPD_ROOT = "/var/lib/nnpd"

var (
	m     map[string]*types.HealthCheck
	mutex = &sync.Mutex{}
)

func init() {
	m = make(map[string]*types.HealthCheck)
}

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

func readConfig(configPath string, configFile interface{}) error {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, configFile)
}

func collect(done chan bool) {
	startServer := false
	configPath := NNPD_ROOT + "/config.json"
	configFile := []types.Config{}
	if err := readConfig(configPath, &configFile); err != nil {
		log.Fatalln(err)
	}

	var wg sync.WaitGroup

	for {
		//fmt.Println("Collecting node health")
		for _, cfg := range configFile {
			wg.Add(1)
			go executeHealthCheck(&wg, cfg)
		}
		wg.Wait()

		if !startServer {
			startServer = true
			done <- startServer
		}
		time.Sleep(3 * time.Second)
	}

}

func executeHealthCheck(wg *sync.WaitGroup, cfg types.Config) {
	defer wg.Done()

	hc := &types.HealthCheck{}
	hc.Type = cfg.Type

	healthCheck := NNPD_ROOT + "/" + cfg.Type + "/" + cfg.HealthCheck
	cmd := exec.Command(healthCheck, "")
	output, err := cmd.CombinedOutput()
	if err != nil {
		hc.Result = "Unhealthy"
		hc.Message = string(output)
	} else {
		hc.Result = "Healthy"
		hc.Message = string(output)
	}

	mutex.Lock()
	m[cfg.Type] = hc
	mutex.Unlock()
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /v1/health/")
	w.WriteHeader(http.StatusOK)
}

func nodeHealthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /v1/nodehealth/")

	res := []types.HealthCheck{}

	mutex.Lock()
	for _, val := range m {
		res = append(res, *val)
	}
	mutex.Unlock()

	respJSON, err := json.Marshal(res)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respJSON)
}
