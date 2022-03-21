package aggregator

import (
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
			Name: "node_handle_errors",
			Help: "Count of errors while handling a node",
		}, []string{"dc"})

	nodeHandleSkipCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_handle_skips",
			Help: "Count of nodes skipped",
		}, []string{"dc"})

	nodeHealthyCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_healthy",
			Help: "Count of healthy nodes",
		}, []string{"dc"})

	nodeUnhealthyCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_unhealthy",
			Help: "Count of unhealthy nodes",
		}, []string{"dc"})

	nodeHealthStateChangedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "node_health_state_changes",
			Help: "Count of hosts whose states has changed",
		}, []string{"dc"})
)

func registerMetrics() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(aggregatorCyclesTotalCounter)
	r.MustRegister(aggregatorProcessingTime)
	r.MustRegister(eligibleNodesGauge)
	r.MustRegister(nodesTotalGauge)
	r.MustRegister(nodeHandleErrorsCounter)
	r.MustRegister(nodeHandleSkipCounter)
	r.MustRegister(nodeHealthyCounter)
	r.MustRegister(nodeUnhealthyCounter)
	r.MustRegister(nodeHealthStateChangedCounter)
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
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Failed to start Prometheus scrape endpoint: %v", err)
		}
	}()
}
