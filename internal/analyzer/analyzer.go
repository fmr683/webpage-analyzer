package analyzer

import (
	"golang.org/x/net/html" 
	"io"                   
	"net/http"              
	"net/url"              
	"strings"               
	"sync"           
	"time"  
)

// AnalysisResult holds everything we learn about a webpage
type AnalysisResult struct {
	HTMLVersion  string          // e.g., "HTML5", "HTML 4.01", or "Unknown"
	Title        string          // Page <title> content
	Headings     map[string]int  // Count of <h1>, <h2>, etc. → e.g., "h1": 2
	Links        Links           // Breakdown of internal/external/inaccessible links
	HasLoginForm bool            // Does the page likely have a login form?
}

// Links categorizes all <a href=""> links on the page
type Links struct {
	Internal     int // Links to same domain (e.g., /about → yoursite.com/about)
	External     int // Links to other domains (e.g., google.com)
	Inaccessible int // Links that are broken or can't be checked
}

// httpClient is a reusable HTTP client with:
// - 5-second timeout (don't hang forever)
// - No redirect following (we just want to know if link works, not where it goes)
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // Stop after first response, don't follow 301/302
	},
}

// Main function: Analyze HTML from a reader (could be file, HTTP response, etc.)
// pageURL is the original URL of the page (needed to resolve relative links)
func AnalyzePage(body io.Reader, pageURL string) (*AnalysisResult, error) {
	// Parse the raw HTML into a DOM tree (like in browser dev tools)
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err // If HTML is broken, bail out
	}

	// Prepare empty result
	result := &AnalysisResult{
		Headings: make(map[string]int), // Initialize empty map for heading counts
	}

	// === DETECT HTML VERSION ===
	// Look at the very first node: it should be <!DOCTYPE ...>
	if doc.FirstChild != nil && doc.FirstChild.Type == html.DoctypeNode {
		// Example: "<!doctype html>" → we just grab "html" and uppercase it
		result.HTMLVersion = strings.ToUpper(doc.FirstChild.Data)
	} else {
		result.HTMLVersion = "Unknown" // No doctype = old or broken HTML
	}

	// === PREPARE VARIABLES FOR TRAVERSAL ===
	var links []string   // Collect all href values
	var hasLogin bool    // Will be true if we find a login-like form

	// === TRAVERSE THE HTML TREE ===
	// This is a recursive function that walks through every node in the DOM
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		// Only care about HTML elements (ignore text, comments, etc.)
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data) // e.g., "H1" → "h1"

			switch tag {
			case "title":
				// <title>Page Title</title> → grab the text inside
				if n.FirstChild != nil {
					result.Title = n.FirstChild.Data
				}

			case "h1", "h2", "h3", "h4", "h5", "h6":
				// Count how many of each heading we see
				// Use lowercase keys so "H1" and "h1" don't get counted separately
				result.Headings[tag]++

			case "a":
				// Found a link: <a href="..."> → extract href attribute
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						links = append(links, attr.Val) // Save the raw href
					}
				}

			case "form":
				// Very simple login form detection:
				// Look for:
				// 1. An <input type="password">
				// 2. An <input> with name/email/login/etc. + type=text/email
				var hasPassword, hasUser bool

				// Recursively check all <input> inside this form
				var checkInputs func(*html.Node)
				checkInputs = func(m *html.Node) {
					if m.Type == html.ElementNode && strings.ToLower(m.Data) == "input" {
						var inputType, inputName string

						// Loop through all attributes of this input
						for _, attr := range m.Attr {
							key := strings.ToLower(attr.Key)
							if key == "type" {
								inputType = strings.ToLower(attr.Val)
							}
							if key == "name" {
								inputName = strings.ToLower(attr.Val)
							}
						}

						// Check for password field
						if inputType == "password" {
							hasPassword = true
						}

						// Check for username/email field by type + name
						if (inputType == "email" || inputType == "text") &&
							(strings.Contains(inputName, "user") ||
								strings.Contains(inputName, "email") ||
								strings.Contains(inputName, "login") ||
								strings.Contains(inputName, "username")) {
							hasUser = true
						}
					}

					// Keep digging into child nodes
					for c := m.FirstChild; c != nil; c = c.NextSibling {
						checkInputs(c)
					}
				}

				// Start checking inputs inside this form
				checkInputs(n)

				// If both user + password fields exist → probably a login form
				if hasPassword && hasUser {
					hasLogin = true
				}
			}
		}

		// Go deeper: visit all children of current node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	// Start traversal from the root of the document
	traverse(doc)

	// Save final results
	result.HasLoginForm = hasLogin
	result.Links = analyzeLinks(links, pageURL) // Now classify and check all links

	return result, nil
}

// analyzeLinks takes raw hrefs and the page's base URL, then:
// 1. Converts relative → absolute URLs
// 2. Classifies internal vs external
// 3. Checks each link with HTTP HEAD request (fast, no body download)
// 4. Counts inaccessible (404, timeout, etc.)
func analyzeLinks(links []string, baseURL string) Links {
	// Parse the main page URL (e.g., "https://example.com/path")
	parsedBase, err := url.Parse(baseURL)
	if err != nil || parsedBase == nil || parsedBase.Host == "" {
		// If base URL is garbage, assume ALL links are inaccessible
		return Links{Inaccessible: len(links)}
	}

	// Thread-safe counters (because we use goroutines)
	var mu sync.Mutex
	var wg sync.WaitGroup

	internalCount := 0
	externalCount := 0
	inaccCount := 0

	// Loop through every link found on the page
	for _, raw := range links {
		// Parse the raw href (could be "/about", "https://google.com", "#top", etc.)
		parsed, err := url.Parse(raw)
		if err != nil {
			inaccCount++ // Can't even parse → inaccessible
			continue
		}

		// Convert to full absolute URL: "/about" → "https://example.com/about"
		abs := parsedBase.ResolveReference(parsed)

		// Skip invalid URLs (missing scheme or host)
		if abs.Scheme == "" || abs.Host == "" {
			inaccCount++
			continue
		}

		// Is this link on the same domain?
		isInternal := abs.Host == parsedBase.Host

		// === CONCURRENCY CONTROL ===
		// Acquire() / Release() likely limit how many requests run at once
		// (Not defined here — probably a semaphore in real code)
		Acquire()
		wg.Add(1)

		// Launch a goroutine to check this one link
		go func(u *url.URL, isInternal bool) {
			defer wg.Done()   // Mark this task done when finished
			defer Release()   // Free up slot for next request

			// Use HEAD request: fast way to check if link works (no HTML body)
			resp, err := httpClient.Head(u.String())

			// Lock counters while updating (thread safety)
			mu.Lock()
			defer mu.Unlock()

			if err != nil || (resp != nil && resp.StatusCode >= 400) {
				// Network error OR 404, 500, etc. → inaccessible
				inaccCount++
			} else {
				// Link works!
				if isInternal {
					internalCount++
				} else {
					externalCount++
				}
			}

			// Always close response body to avoid leaks
			if resp != nil {
				resp.Body.Close()
			}
		}(abs, isInternal) // Pass variables into goroutine
	}

	// Wait for all link checks to finish
	wg.Wait()

	// Return final counts
	return Links{
		Internal:     internalCount,
		External:     externalCount,
		Inaccessible: inaccCount,
	}
}