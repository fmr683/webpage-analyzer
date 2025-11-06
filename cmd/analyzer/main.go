package main

import (
	"net/http"

	"github.com/sirupsen/logrus"

	"webpage-analyzer/internal/analyzer"
)

var logger = logrus.New()

func main() {
	// Use JSON formatted logs (nice for production, easy to read in dev)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Serve static assets (HTML + CSS)
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/", fs)

	// Analysis endpoint
	http.HandleFunc("/analyze", analyzer.AnalyzeHandler(logger))

	logger.Info("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		logger.WithError(err).Fatal("Server failed")
	}
}