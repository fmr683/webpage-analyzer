package analyzer

import (
	"fmt"
	"net/http"
	"regexp"
)

// Basic URL regex for validation
var urlRegex = regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)

func AnalyzeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	if !urlRegex.MatchString(url) {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching URL: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("URL unreachable: HTTP %d - %s", resp.StatusCode, http.StatusText(resp.StatusCode)), resp.StatusCode)
		return
	}

	// Placeholder response
	fmt.Fprintf(w, "Fetched URL: %s\nStatus: %d", url, resp.StatusCode)
}