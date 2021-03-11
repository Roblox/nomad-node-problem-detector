package detector

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	types "github.com/nomad-node-problem-detector/types"
	"github.com/urfave/cli/v2"
)

var (
	m        map[string]*types.HealthCheck
	mutex    = &sync.Mutex{}
	nnpdRoot = "/var/lib/nnpd"
)

//Todo: Add comments to describe locking/contention.
func init() {
	m = make(map[string]*types.HealthCheck)
}

var DetectorCommand = &cli.Command{
	Name:  "detector",
	Usage: "Run nomad node problem detector HTTP server",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "detector-cycle-time",
			Aliases: []string{"t"},
			Value:   "3s",
			Usage:   "Time (in seconds) to wait between each detector cycle",
		},
		&cli.StringFlag{
			Name:    "root-dir",
			Aliases: []string{"d"},
			Usage:   "Location of health checks. Defaults to /var/lib/nnpd",
		},
	},
	Action: func(c *cli.Context) error {
		return startNpdHttpServer(c)
	},
}

func startNpdHttpServer(context *cli.Context) error {
	log.Info("Starting nomad node problem detector...")
	detectorCycleTime, err := time.ParseDuration(context.String("detector-cycle-time"))
	if err != nil {
		return err
	}

	rootDir := context.String("root-dir")
	if rootDir != "" {
		nnpdRoot = rootDir
	}

	nomadAllocDir := os.Getenv("NOMAD_ALLOC_DIR")
	if nomadAllocDir != "" {
		nnpdRoot = nomadAllocDir + nnpdRoot
	}

	done := make(chan bool, 1)
	go collect(done, detectorCycleTime)
	<-done

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to Nomad node problem detector!\n"))
	})
	http.HandleFunc("/v1/health/", healthCheckHandler)
	http.HandleFunc("/v1/nodehealth/", nodeHealthHandler)

	log.Info("nomad node problem detector ready to receive requests.")
	if err := http.ListenAndServe(":8083", nil); err != nil {
		return err
	}
	return nil
}

func readConfig(configPath string, configFile interface{}) error {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			msg := fmt.Sprintf("Config file: %s does not exist, continue with default checks.\n", configPath)
			log.Warning(msg)
			return nil
		} else {
			return err
		}
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, configFile)
}

func collect(done chan bool, detectorCycleTime time.Duration) {
	startServer := false
	configPath := nnpdRoot + "/config.json"

	var wg sync.WaitGroup

	configFile := []types.Config{}
	if err := readConfig(configPath, &configFile); err != nil {
		log.Fatal(err)
	}

	for {
		for _, cfg := range configFile {
			wg.Add(1)
			go executeHealthCheck(&wg, cfg)
		}
		wg.Wait()

		getDiskStats()

		if !startServer {
			startServer = true
			done <- startServer
		}
		time.Sleep(detectorCycleTime)
	}

}

func getDiskStats() {
	hc := &types.HealthCheck{}
	hc.Type = "DiskUnderPressure"

	diskStats, err := collectDiskStats()
	if err != nil {
		hc.Result = "true"
		hc.Message = err.Error()
	}

	log.Info("Disk used percent: %f\n", diskStats.UsedPercent)
}

func executeHealthCheck(wg *sync.WaitGroup, cfg types.Config) {
	defer wg.Done()

	hc := &types.HealthCheck{}
	hc.Type = cfg.Type

	healthCheck := nnpdRoot + "/" + cfg.Type + "/" + cfg.HealthCheck
	cmd := exec.Command(healthCheck, "")

	var output bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		hc.Result = "Unhealthy"
		hc.Message = fmt.Sprintf("%s:%s\n", err.Error(), stderr.String())
	} else {
		hc.Result = "Healthy"
		hc.Message = output.String()
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
