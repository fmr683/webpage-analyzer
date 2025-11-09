package main

import (
	"net/http"

	"github.com/didip/tollbooth/v7"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"webpage-analyzer/internal/analyzer"
)

var logger = logrus.New()

func main() {
	// === Logging ===
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// === Template & Metrics Init ===
	analyzer.Tmpl = analyzer.LoadTemplate()
	analyzer.InitMetrics() // ← NEW: Register Prometheus metrics

	// === Static Files ===
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	// === Rate Limiting (5 req/sec per IP) ===
	limiter := tollbooth.NewLimiter(5, nil) // 5 requests per second per IP
	http.Handle("/analyze",
		tollbooth.LimitFuncHandler(limiter, analyzer.AnalyzeHandler(logger)),
	)

	// === Prometheus Metrics Endpoint ===
	http.Handle("/metrics", promhttp.Handler())

	// === Start Server ===
	logger.Info("Server starting on :8080")
	logger.Info("Metrics available at http://localhost:8080/metrics")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.WithError(err).Fatal("Server failed")
	}
}