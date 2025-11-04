package analyzer

import (
	"golang.org/x/net/html"
	"io"
	"strings"
)

type AnalysisResult struct {
	HTMLVersion string
	Title       string
	Headings    map[string]int // e.g., "h1": 2
}

func AnalyzePage(body io.Reader) (*AnalysisResult, error) {
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

	// Traverse for title and headings
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if n.FirstChild != nil {
					result.Title = n.FirstChild.Data
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				result.Headings[n.Data]++
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	return result, nil
}