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

type AnalysisResult struct {
	HTMLVersion  string
	Title        string
	Headings     map[string]int // e.g., "h1": 2
	Links        Links
	HasLoginForm bool
}

type Links struct {
	Internal     int
	External     int
	Inaccessible int
}

// Mockable HTTP client (timeout + no-follow redirects)
var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func AnalyzePage(body io.Reader, pageURL string) (*AnalysisResult, error) {
	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	result := &AnalysisResult{
		Headings: make(map[string]int),
	}

	// Detect HTML version (simple: check doctype)
	if doc.FirstChild != nil && doc.FirstChild.Type == html.DoctypeNode {
		result.HTMLVersion = strings.ToUpper(doc.FirstChild.Data)
	} else {
		result.HTMLVersion = "Unknown"
	}

	// Traverse for title, headings, links, and login forms
	var links []string
	var hasLogin bool

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if n.FirstChild != nil {
					result.Title = n.FirstChild.Data
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				// ensure keys are lowercase and consistent
				result.Headings[strings.ToLower(n.Data)]++
			case "a":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						links = append(links, attr.Val)
					}
				}
			case "form":
				// very simple login-form heuristic
				var hasPassword, hasUser bool
				var checkInputs func(*html.Node)
				checkInputs = func(m *html.Node) {
					if m.Type == html.ElementNode && strings.ToLower(m.Data) == "input" {
						var inputType, inputName string
						for _, attr := range m.Attr {
							switch strings.ToLower(attr.Key) {
							case "type":
								inputType = strings.ToLower(attr.Val)
							case "name":
								inputName = strings.ToLower(attr.Val)
							}
						}
						if inputType == "password" {
							hasPassword = true
						}
						if (inputType == "email" || inputType == "text") &&
							(strings.Contains(inputName, "user") ||
								strings.Contains(inputName, "email") ||
								strings.Contains(inputName, "login") ||
								strings.Contains(inputName, "username")) {
							hasUser = true
						}
					}
					for c := m.FirstChild; c != nil; c = c.NextSibling {
						checkInputs(c)
					}
				}
				checkInputs(n)
				if hasPassword && hasUser {
					hasLogin = true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	result.HasLoginForm = hasLogin
	result.Links = analyzeLinks(links, pageURL)

	return result, nil
}

func analyzeLinks(links []string, baseURL string) Links {
	parsedBase, err := url.Parse(baseURL)
	if err != nil || parsedBase == nil || parsedBase.Host == "" {
		return Links{Inaccessible: len(links)}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	internalCount := 0
	externalCount := 0
	inaccCount := 0

	for _, raw := range links {
		parsed, err := url.Parse(raw)
		if err != nil {
			inaccCount++
			continue
		}
		abs := parsedBase.ResolveReference(parsed)
		if abs.Scheme == "" || abs.Host == "" {
			inaccCount++
			continue
		}

		isInternal := abs.Host == parsedBase.Host

		Acquire()
		wg.Add(1)
		go func(u *url.URL, isInternal bool) {
			defer wg.Done()
			defer Release()

			resp, err := httpClient.Head(u.String())
			mu.Lock()
			defer mu.Unlock()

			if err != nil || (resp != nil && resp.StatusCode >= 400) {
				inaccCount++
			} else {
				if isInternal {
					internalCount++
				} else {
					externalCount++
				}
			}
			if resp != nil {
				resp.Body.Close()
			}
		}(abs, isInternal)
	}
	wg.Wait()

	return Links{
		Internal:     internalCount,
		External:     externalCount,
		Inaccessible: inaccCount,
	}
}
