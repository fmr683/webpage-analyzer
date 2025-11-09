package analyzer

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"

	"github.com/sirupsen/logrus"
)

type pageData struct {
	URL          string
	HTMLVersion  string
	Title        string
	Headings     map[string]int
	Links        Links
	HasLoginForm bool
	Error        string
}

var (
	urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	Tmpl     *template.Template // **do NOT initialise here**
)

// LoadTemplate is called from main.go (or wherever you start the server)
func LoadTemplate() *template.Template {
	t, err := template.ParseFiles("static/results.html")
	if err != nil {
		panic(fmt.Sprintf("failed to load template: %v", err))
	}
	return t
}

// AnalyzeHandler creates an HTTP handler function that processes webpage analysis requests
// It takes a logger (logrus) so we can record what’s happening (for debugging or monitoring)
func AnalyzeHandler(log *logrus.Logger) http.HandlerFunc {

	// Return a function that matches http.HandlerFunc signature:
	// func(w http.ResponseWriter, r *http.Request)
	return func(w http.ResponseWriter, r *http.Request) {

		// === STEP 1: Only allow POST requests ===
		// If someone tries GET, PUT, etc. → reject them
		if r.Method != http.MethodPost {
			// 405 = "Method Not Allowed"
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return // Stop here
		}

		// === STEP 2: Get the URL from form data ===
		// User should send: <form><input name="url" value="https://example.com"></form>
		rawURL := r.FormValue("url")
		if rawURL == "" {
			// No URL provided → show friendly error
			renderError(w, "URL is required")
			return
		}

		// === STEP 3: Validate URL format using regex ===
		// urlRegex is probably defined elsewhere like: ^https?://.*
		// It makes sure the URL looks valid (http:// or https://, etc.)
		if !urlRegex.MatchString(rawURL) {
			renderError(w, "Invalid URL format")
			return
		}

		// === STEP 4: Log that we're starting analysis ===
		// This helps developers see what's happening in logs
		log.WithFields(logrus.Fields{
			"url": rawURL,
		}).Info("Starting analysis")

		// === STEP 5: Download the webpage ===
		resp, err := http.Get(rawURL) // Simple HTTP GET request
		if err != nil {
			// Network error, timeout, bad domain, etc.
			renderError(w, fmt.Sprintf("Failed to fetch URL: %v", err))
			return
		}
		// Always close the response body to prevent memory leaks
		defer resp.Body.Close()

		// === STEP 6: Check if page loaded successfully (200 OK) ===
		if resp.StatusCode != http.StatusOK {
			// 404, 500, 403, etc. → page not available
			renderError(w, fmt.Sprintf("URL unreachable – HTTP %d %s", 
				resp.StatusCode, 
				http.StatusText(resp.StatusCode))) // e.g., "404 Not Found"
			return
		}

		// === STEP 7: Parse the HTML and analyze it ===
		// This uses the AnalyzePage function from earlier
		result, err := AnalyzePage(resp.Body, rawURL)
		if err != nil {
			// HTML is broken, malformed, etc.
			renderError(w, fmt.Sprintf("HTML parsing error: %v", err))
			return
		}

		// === STEP 8: Prepare data to show in HTML template ===
		data := pageData{
			URL:          rawURL,
			HTMLVersion:  result.HTMLVersion,  // e.g., "HTML5"
			Title:        result.Title,        // <title> content
			Headings:     result.Headings,     // {"h1": 1, "h2": 3, ...}
			Links:        result.Links,        // internal/external/broken counts
			HasLoginForm: result.HasLoginForm, // true if login form detected
		}

		// === STEP 9: Render the result using an HTML template ===
		// Tmpl is a global *html/template.Template defined elsewhere
		if err := Tmpl.Execute(w, data); err != nil {
			// If template fails (syntax error, missing field, etc.)
			log.WithError(err).Error("Template render failed")
			// Show generic error to user (don't leak details)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// If we get here → everything worked! User sees nice report
	}
}

func renderError(w http.ResponseWriter, msg string) {
	data := pageData{Error: msg}
	_ = Tmpl.Execute(w, data) // ignore error – we are already in an error path
}