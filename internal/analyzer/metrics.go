package analyzer

import "github.com/prometheus/client_golang/prometheus"

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "analyzer_requests_total", Help: "Total requests"},
		[]string{"status"},
	)
	AnalysisDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "analyzer_duration_seconds",
		Help:    "Analysis duration",
		Buckets: prometheus.DefBuckets,
	})
)

func InitMetrics() {
	prometheus.MustRegister(RequestsTotal, AnalysisDuration)
}