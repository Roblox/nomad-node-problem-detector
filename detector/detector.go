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

package detector

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	units "github.com/docker/go-units"
	"github.com/nomad-node-problem-detector/detector/logwatchers"
	types "github.com/nomad-node-problem-detector/types"
	"github.com/urfave/cli/v2"
)

type Limits struct {
	cpuLimit    string
	memoryLimit string
	diskLimit   string
}

var (
	m                 map[string]*types.HealthCheck
	mutex             = &sync.Mutex{}
	nnpdRoot          = "/var/lib/nnpd"
	detectorHTTPToken string
	auth              bool

	logWatcherProblemCounter = &prometheus.CounterVec{}
)

//Todo: Add comments to describe locking/contention.
func init() {
	m = make(map[string]*types.HealthCheck)
}

var DetectorCommand = &cli.Command{
	Name:  "detector",
	Usage: "Run nomad node problem detector HTTP server",
	Flags: []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "log-monitor",
			Usage: "List of configuration file name for monitoring logs. Can be repeated multiple time, and is relative to the root directory.",
		},
		&cli.StringFlag{
			Name:    "detector-cycle-time",
			Aliases: []string{"t"},
			Value:   "3s",
			Usage:   "Time (in seconds) to wait between each detector cycle",
		},
		&cli.StringFlag{
			Name:    "port",
			Aliases: []string{"p"},
			Value:   ":8083",
			Usage:   "Address to listen on for detector HTTP server",
		},
		&cli.BoolFlag{
			Name:  "auth",
			Usage: "If set to true, detector must set DETECTOR_HTTP_TOKEN=<your_token> as an environment variable when starting detector",
		},
		&cli.StringFlag{
			Name:    "root-dir",
			Aliases: []string{"d"},
			Usage:   "Location of health checks. Defaults to /var/lib/nnpd",
		},
		&cli.StringFlag{
			Name:    "cpu-limit",
			Aliases: []string{"cl"},
			Value:   "85",
			Usage:   "CPU threshold in percentage",
		},
		&cli.StringFlag{
			Name:    "memory-limit",
			Aliases: []string{"ml"},
			Value:   "80",
			Usage:   "Memory threshold in percentage",
		},
		&cli.StringFlag{
			Name:    "disk-limit",
			Aliases: []string{"dl"},
			Value:   "90",
			Usage:   "Disk threshold in percentage",
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

	auth = context.Bool("auth")
	if auth {
		detectorHTTPToken = os.Getenv("DETECTOR_HTTP_TOKEN")
		if detectorHTTPToken == "" {
			return fmt.Errorf("the environment variable `DETECTOR_HTTP_TOKEN' is missing, with --auth enabled")
		}
	}

	rootDir := context.String("root-dir")
	if rootDir != "" {
		nnpdRoot = rootDir
	}

	limits := &Limits{
		cpuLimit:    context.String("cpu-limit"),
		memoryLimit: context.String("memory-limit"),
		diskLimit:   context.String("disk-limit"),
	}

	reg := registerMetrics()

	nomadAllocDir := os.Getenv("NOMAD_ALLOC_DIR")
	if nomadAllocDir != "" {
		nnpdRoot = nomadAllocDir + nnpdRoot
	}

	done := make(chan bool, 1)
	go collect(done, detectorCycleTime, limits)
	<-done

	if err := SetupLogMonitor(context.StringSlice("log-monitor")); err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Welcome to Nomad node problem detector!\n"))
	})
	http.HandleFunc("/v1/health/", healthCheckHandler)
	http.HandleFunc("/v1/nodehealth/", nodeHealthHandler)
	http.Handle("/v1/metrics/", metricsHandler(reg))

	log.Info(fmt.Sprintf("detector started with --cpu-limit: %s%%", limits.cpuLimit))
	log.Info(fmt.Sprintf("detector started with --memory-limit: %s%%", limits.memoryLimit))
	log.Info(fmt.Sprintf("detector started with --disk-limit: %s%%", limits.diskLimit))

	port := context.String("port")
	log.Info(fmt.Sprintf("nomad node problem detector ready to receive requests. Listening on %s", port))
	if err := http.ListenAndServe(port, nil); err != nil {
		return err
	}
	return nil
}

func readConfig(configPath string, configFile interface{}) error {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file: %s does not exist", configPath)
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

func collect(done chan bool, detectorCycleTime time.Duration, limits *Limits) {
	startServer := false
	configPath := nnpdRoot + "/config.json"

	var wg sync.WaitGroup

	configFile := []types.Config{}
	if err := readConfig(configPath, &configFile); err != nil {
		msg := fmt.Sprintf("Error in reading config: %s: error: %s, continue with default cpu, memory and disk checks.\n", configPath, err.Error())
		log.Warning(msg)
	}

	cpuLimit, err := strconv.ParseFloat(limits.cpuLimit, 64)
	if err != nil {
		errMsg := fmt.Sprintf("Error in parsing --cpu-limit: %s", err.Error())
		log.Fatal(errMsg)
	}

	memoryLimit, err := strconv.ParseFloat(limits.memoryLimit, 64)
	if err != nil {
		errMsg := fmt.Sprintf("Error in parsing --memory-limit: %s", err.Error())
		log.Fatal(errMsg)
	}

	diskLimit, err := strconv.ParseFloat(limits.diskLimit, 64)
	if err != nil {
		errMsg := fmt.Sprintf("Error in parsing --disk-limit: %s", err.Error())
		log.Fatal(errMsg)
	}

	for {
		for _, cfg := range configFile {
			wg.Add(1)
			go executeHealthCheck(&wg, cfg)
		}
		wg.Wait()

		getCPUStats(cpuLimit)
		getMemoryStats(memoryLimit)
		getDiskStats(diskLimit)

		if !startServer {
			startServer = true
			done <- startServer
		}
		time.Sleep(detectorCycleTime)
	}

}

// Get CPU usage of the nomad client node.
func getCPUStats(cpuLimit float64) {
	hc := &types.HealthCheck{}
	hc.Type = "CPUUnderPressure"

	cpuStats, err := collectCPUStats()
	if err != nil {
		hc.Update("true", err.Error())
	} else if cpuStats.User >= cpuLimit {
		hc.Update("true", fmt.Sprintf("CPU usage: %f %%", cpuStats.User))
	} else {
		hc.Update("false", fmt.Sprintf("CPU usage: %f %%", cpuStats.User))
	}

	mutex.Lock()
	m[hc.Type] = hc
	mutex.Unlock()
}

// Get memory usage of the nomad client node.
func getMemoryStats(memoryLimit float64) {
	hc := &types.HealthCheck{}
	hc.Type = "MemoryUnderPressure"

	memoryAvailableLimit := (100 - memoryLimit)

	memoryStats, err := collectMemoryStats()
	if err != nil {
		hc.Update("true", err.Error())
	} else {
		availableMemory := units.HumanSize(float64(memoryStats.Available))
		availableMemoryPercent := (float64(memoryStats.Available) / float64(memoryStats.Total)) * 100
		totalMemory := units.HumanSize(float64(memoryStats.Total))
		var result string
		if availableMemoryPercent <= memoryAvailableLimit {
			result = "true"
		} else {
			result = "false"
		}
		hc.Update(result, fmt.Sprintf("%s memory available out of %s total memory", availableMemory, totalMemory))
	}

	mutex.Lock()
	m[hc.Type] = hc
	mutex.Unlock()
}

// Get disk usage of the nomad client node.
func getDiskStats(diskLimit float64) {
	hc := &types.HealthCheck{}
	hc.Type = "DiskUsageHigh"

	diskStats, err := collectDiskStats()
	if err != nil {
		hc.Update("true", err.Error())
	} else if diskStats.UsedPercent >= diskLimit {
		hc.Update("true", fmt.Sprintf("disk usage is %f %%", diskStats.UsedPercent))
	} else {
		hc.Update("false", fmt.Sprintf("disk usage is %f %%", diskStats.UsedPercent))
	}

	mutex.Lock()
	m[hc.Type] = hc
	mutex.Unlock()
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
		hc.Update("Unhealthy", fmt.Sprintf("%s:%s\n", err.Error(), stderr.String()))
	} else {
		hc.Update("Healthy", output.String())
	}

	mutex.Lock()
	m[cfg.Type] = hc
	mutex.Unlock()
}

func validateAuthorizationToken(w http.ResponseWriter, r *http.Request) error {
	response := r.Header.Get("Authorization")
	tokens := strings.Split(response, " ")
	if len(tokens) < 2 {
		return fmt.Errorf("malformed or missing token in http request header")
	}

	requestToken := tokens[1]
	token := base64.StdEncoding.EncodeToString([]byte(detectorHTTPToken))

	if token != requestToken {
		return fmt.Errorf("invalid token in http request header")
	}
	return nil
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /v1/health/")
	if auth {
		if err := validateAuthorizationToken(w, r); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func nodeHealthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /v1/nodehealth/")
	if auth {
		if err := validateAuthorizationToken(w, r); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}
	}

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

// SetupLogMonitor configures the various log monitors based on the
// provided configuration. For each of the monitors, we start a
// watcher which will receive events that are matching the provided
// rules.
func SetupLogMonitor(logMonitorsConfigFile []string) error {
	for _, cfgFilename := range logMonitorsConfigFile {
		cfgFilePath := path.Join(nnpdRoot, cfgFilename)
		loggingMonitorConfig := &types.LogWatcherConfig{}
		if err := readConfig(cfgFilePath, loggingMonitorConfig); err != nil {
			return fmt.Errorf("error in reading config %s: %s", cfgFilePath, err)
		}
		watcher, err := logwatchers.GetLogWatcher(loggingMonitorConfig)
		if err != nil {
			return err
		}
		go logMonitor(watcher)
	}
	return nil
}

// logMonitor receives events from the various log monitors and handle
// these events.
func logMonitor(w types.LogWatcher) {
	logCh := w.Watch()
	for {
		got := <-logCh
		log.Infof("log watcher received for %s: %s", got.Name, got.Message)
		logWatcherProblemCounter.With(prometheus.Labels{"check": got.Name}).Inc()
	}
}

func metricsHandler(registry *prometheus.Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth {
			if err := validateAuthorizationToken(w, r); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(err.Error()))
				return
			}
		}
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	})
}

func registerMetrics() *prometheus.Registry {
	counterOpts := prometheus.CounterOpts{}

	if nomadDc, ok := os.LookupEnv("NOMAD_DC"); ok {
		counterOpts.ConstLabels = prometheus.Labels{"nomad_dc": nomadDc}
	}

	counterOpts.Name = "npd_detector_log_problem_count"
	counterOpts.Help = "Number of time a specific log problem was reported"
	logWatcherProblemCounter = prometheus.NewCounterVec(counterOpts, []string{"check"})

	r := prometheus.NewRegistry()
	r.MustRegister(prometheus.NewGoCollector())
	r.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	r.MustRegister(logWatcherProblemCounter)
	return r
}
