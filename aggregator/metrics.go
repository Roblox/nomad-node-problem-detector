package aggregator

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	aggregatorCyclesTotalCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aggregator_cycles_total",
			Help: "Number of cycles",
		}, []string{"dc"})

	aggregatorProcessingTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aggregator_processing_time",
			Help: "time spend aggregating information for this cycle",
		}, []string{"dc"})

	nodesTotalGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nodes_total",
			Help: "Total number of nodes",
		}, []string{"dc"})

	eligibleNodesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nodes_eligible",
			Help: "Number of eligible nodes for this cycle",
		}, []string{"dc"})

	nodeHandleErrorsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nodes_handle_errors",
			Help: "Count of errors while handling nodes",
		}, []string{"dc"})

	nodeHandleSkipCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nodes_handle_skip",
			Help: "Count of nodes skipped",
		}, []string{"dc"})

	healthCheckHealthyCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nodes_healthy",
			Help: "Count of healthy nodes",
		}, []string{"dc", "check"})

	healthCheckUnhealthyCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "nodes_unhealthy",
			Help: "Count of unhealthy nodes",
		}, []string{"dc", "check", "host"})
)

func registerMetrics() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(aggregatorCyclesTotalCounter)
	r.MustRegister(aggregatorProcessingTime)
	r.MustRegister(eligibleNodesGauge)
	r.MustRegister(nodesTotalGauge)
	r.MustRegister(nodeHandleErrorsCounter)
	r.MustRegister(nodeHandleSkipCounter)
	r.MustRegister(healthCheckHealthyCounter)
	r.MustRegister(healthCheckUnhealthyCounter)
	r.MustRegister(prometheus.NewGoCollector())
	r.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return r
}

func metricsExporter(exporterAddr string, exporterPort int) {
	addr := net.JoinHostPort(exporterAddr, strconv.Itoa(exporterPort))

	registry := registerMetrics()
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		mux.HandleFunc("/health", healthCheckHandler)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Failed to start Prometheus scrape endpoint: %v", err)
		}
	}()
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Calling /health")
	w.WriteHeader(http.StatusOK)
}
