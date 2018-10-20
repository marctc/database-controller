package metrics

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ControllerMetrics contains Prometheus metrics for the database controller.
type ControllerMetrics struct {
	metrics map[string]prometheus.Collector
}

const (
	metricNamespace = "database_controller"
	metricsEndpoint = "/metrics"
	metricsAddress  = ":8080"
)

var (
	databaseLabels = []string{"database_engine", "database_host", "database_name"}
)

// NewControllerMetrics returns new ControllerMetrics
func NewControllerMetrics() *ControllerMetrics {
	return &ControllerMetrics{
		metrics: map[string]prometheus.Collector{
			"databaseCreated": prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: metricNamespace,
					Name:      "database_created",
					Help:      "Creation timestamp",
				}, databaseLabels,
			),
			"databaseDeleted": prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: metricNamespace,
					Name:      "database_deleted",
					Help:      "Deletion timestamp",
				}, databaseLabels,
			),
			"databaseCreationFailure": prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      "database_creation_failure",
					Help:      "Total number of database creation failures",
				}, databaseLabels,
			),
			"databaseDeletionFailure": prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: metricNamespace,
					Name:      "database_deletion_failure",
					Help:      "Total number of database deletion failures",
				}, databaseLabels,
			),
		},
	}
}

// RegisterAllMetrics registers all Prometheus metrics
func (cm *ControllerMetrics) RegisterAllMetrics() {
	for _, metric := range cm.metrics {
		prometheus.MustRegister(metric)
	}
}

// RunServer runs a HTTP server exposing Prometheus metrics
func (cm *ControllerMetrics) RunServer() {
	metricsMux := http.NewServeMux()
	metricsMux.Handle(metricsEndpoint, promhttp.Handler())
	log.Printf("Starting Prometheus metric server at address [%s]", metricsAddress)
	if err := http.ListenAndServe(metricsAddress, metricsMux); err != nil {
		log.Fatalf("Failed to start metric server: %v", err)
	}
}

// RegisterDatabaseCreated records the database creation timestamp
func (cm *ControllerMetrics) RegisterDatabaseCreated(engine, host, name string) {
	if g, ok := cm.metrics["databaseCreated"].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(engine, host, name).SetToCurrentTime()
	}
}

// RegisterDatabaseDelete records the database deletion timestamp
func (cm *ControllerMetrics) RegisterDatabaseDelete(engine, host, name string) {
	if g, ok := cm.metrics["databaseDelete"].(*prometheus.GaugeVec); ok {
		g.WithLabelValues(engine, host, name).SetToCurrentTime()
	}
}

// CountDatabaseCreationFailure counts if a database creation fails
func (cm *ControllerMetrics) CountDatabaseCreationFailure(engine, host, name string) {
	if c, ok := cm.metrics["databaseCreationFailure"].(*prometheus.CounterVec); ok {
		c.WithLabelValues(engine, host, name).Inc()
	}
}

// CountDatabaseDeletionFailure counts if a database deletion fails
func (cm *ControllerMetrics) CountDatabaseDeletionFailure(engine, host, name string) {
	if c, ok := cm.metrics["databaseDeletionFailure"].(*prometheus.CounterVec); ok {
		c.WithLabelValues(engine, host, name).Inc()
	}
}
