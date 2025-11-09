package main

import (
	"net/http"
	"os"
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

	// === Config from ENV ===
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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
	logger.Infof("Server starting on :%s", port)
	logger.Info("Metrics: http://localhost:" + port + "/metrics")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.WithError(err).Fatal("Server failed")
	}
}