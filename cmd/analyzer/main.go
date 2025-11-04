package main

import (
	"log"
	"net/http"
)

func main() {
	// Serve static files (HTML form)
	http.Handle("/", http.FileServer(http.Dir("static")))

	// Placeholder for analysis handler
	http.HandleFunc("/analyze", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Analysis endpoint - not implemented yet"))
	})

	log.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}