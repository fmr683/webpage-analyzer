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

// Add this global variable for mocking in tests
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

	// Traverse for title, headings, links
	var links []string
	var traverse func(*html.Node)
traverse = func(n *html.Node) {
	if n.Type == html.ElementNode {
		switch strings.ToLower(n.Data) {
		case "title":
			if n.FirstChild != nil {
				result.Title = n.FirstChild.Data
			}
		case "h1", "h2", "h3", "h4", "h5", "h6":
			// keep keys consistent in lowercase
			result.Headings[strings.ToLower(n.Data)]++
		case "a":
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					links = append(links, attr.Val)
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		traverse(c)
	}
}

	traverse(doc)

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
    inacc := 0
    inter := 0
    exter := 0

    for _, rawLink := range links {
        parsed, err := url.Parse(rawLink)
        if err != nil {
            mu.Lock()
            inacc++
            mu.Unlock()
            continue
        }

        absURL := parsedBase.ResolveReference(parsed)
        if absURL.Scheme == "" || absURL.Host == "" {
            mu.Lock()
            inacc++
            mu.Unlock()
            continue
        }

        isInternal := absURL.Host == parsedBase.Host

        wg.Add(1)
        go func(u *url.URL, internal bool) {
            defer wg.Done()
            resp, err := httpClient.Head(u.String()) // uses mockable client
            mu.Lock()
            defer mu.Unlock()
            if err != nil || (resp != nil && resp.StatusCode >= 400) {
                inacc++
            } else {
                if internal {
                    inter++
                } else {
                    exter++
                }
            }
            if resp != nil {
                resp.Body.Close()
            }
        }(absURL, isInternal)
    }
    wg.Wait()

    return Links{
        Internal:     inter,
        External:     exter,
 
		Inaccessible: inacc,
    }
}