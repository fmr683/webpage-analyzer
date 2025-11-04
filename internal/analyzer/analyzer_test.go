package analyzer

import (
	"strings"
	"testing"
)

func TestAnalyzePage(t *testing.T) {
	htmlStr := `<!DOCTYPE html><html><head><title>Test</title></head><body><h1>One</h1><h2>Two</h2></body></html>`
	reader := strings.NewReader(htmlStr)
	result, err := AnalyzePage(reader)
	if err != nil {
		t.Fatal(err)
	}
	if result.HTMLVersion != "HTML" {
		t.Errorf("got %s, want HTML", result.HTMLVersion)
	}
	if result.Title != "Test" {
		t.Errorf("got %s, want Test", result.Title)
	}
	if result.Headings["h1"] != 1 || result.Headings["h2"] != 1 {
		t.Errorf("got %+v, want h1:1, h2:1", result.Headings)
	}
}