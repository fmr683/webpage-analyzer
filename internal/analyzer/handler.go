package analyzer

import (
	"fmt"
	"html/template"
	"net/http"
	"regexp"

	"github.com/sirupsen/logrus"
)

var (
	urlRegex = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	tmpl     = template.Must(template.ParseFiles("static/results.html"))
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

func AnalyzeHandler(log *logrus.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rawURL := r.FormValue("url")
		if rawURL == "" {
			renderError(w, "URL is required")
			return
		}
		if !urlRegex.MatchString(rawURL) {
			renderError(w, "Invalid URL format")
			return
		}

		log.WithFields(logrus.Fields{
			"url": rawURL,
		}).Info("Starting analysis")

		resp, err := http.Get(rawURL)
		if err != nil {
			renderError(w, fmt.Sprintf("Failed to fetch URL: %v", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			renderError(w, fmt.Sprintf("URL unreachable – HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode)))
			return
		}

		result, err := AnalyzePage(resp.Body, rawURL)
		if err != nil {
			renderError(w, fmt.Sprintf("HTML parsing error: %v", err))
			return
		}

		data := pageData{
			URL:          rawURL,
			HTMLVersion:  result.HTMLVersion,
			Title:        result.Title,
			Headings:     result.Headings,
			Links:        result.Links,
			HasLoginForm: result.HasLoginForm,
		}

		if err := tmpl.Execute(w, data); err != nil {
			log.WithError(err).Error("Template render failed")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

func renderError(w http.ResponseWriter, msg string) {
	data := pageData{Error: msg}
	_ = tmpl.Execute(w, data) // ignore error – we are already in an error path
}