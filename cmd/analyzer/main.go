package main

import (
	"log"
	"net/http"
	"webpage-analyzer/internal/analyzer"
)

func main() {
	// Serve static files (HTML form)
	http.Handle("/", http.FileServer(http.Dir("static")))

	http.HandleFunc("/analyze", analyzer.AnalyzeHandler)

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}