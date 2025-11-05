package analyzer

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockTransport returns a Transport that calls fn for each request
type mockTransport func(*http.Request) *http.Response

func (m mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req), nil
}

func TestAnalyzePage_BasicsAndLinks(t *testing.T) {
	// Save original, restore after
	oldClient := httpClient
	defer func() { httpClient = oldClient }()

	// Mock: all URLs return 200 OK
	httpClient = &http.Client{
		Transport: mockTransport(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}
		}),
	}

	htmlStr := `<!DOCTYPE html>
<html><head><title>Test Page</title></head><body>
<h1>A</h1><h2>B</h2><h3>C</h3>
<a href="/internal">int</a>
<a href="https://example.com/ext">ext</a>
<a href=":invalid">bad</a>
</body></html>`

	reader := strings.NewReader(htmlStr)
	result, err := AnalyzePage(reader, "https://mydomain.com/page")
	if err != nil {
		t.Fatal(err)
	}

	if result.HTMLVersion != "HTML" {
		t.Errorf("HTMLVersion = %q; want %q", result.HTMLVersion, "HTML")
	}
	if result.Title != "Test Page" {
		t.Errorf("Title = %q; want %q", result.Title, "Test Page")
	}

	// Headings keys should be lowercase: "h1","h2","h3"
	wantHeadings := map[string]int{"h1": 1, "h2": 1, "h3": 1}
	if len(result.Headings) != len(wantHeadings) {
		t.Errorf("headings count wrong: %+v", result.Headings)
	}
	for k, v := range wantHeadings {
		if result.Headings[k] != v {
			t.Errorf("heading %s = %d; want %d", k, result.Headings[k], v)
		}
	}

	// 1 internal (200), 1 external (200), 1 inaccessible (invalid)
	if result.Links.Internal != 1 {
		t.Errorf("Internal = %d; want 1", result.Links.Internal)
	}
	if result.Links.External != 1 {
		t.Errorf("External = %d; want 1", result.Links.External)
	}
	if result.Links.Inaccessible != 1 {
		t.Errorf("Inaccessible = %d; want 1", result.Links.Inaccessible)
	}

	// No login form here
	if result.HasLoginForm {
		t.Errorf("HasLoginForm = true; want false")
	}
}

func TestAnalyzePage_LoginFormDetection(t *testing.T) {
	// Save original, restore after
	oldClient := httpClient
	defer func() { httpClient = oldClient }()

	// No links in this HTML, but set a harmless mock anyway
	httpClient = &http.Client{
		Transport: mockTransport(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}
		}),
	}

	htmlStr := `
<!DOCTYPE html>
<html>
<head><title>Login Test</title></head>
<body>
  <form action="/login" method="post">
    <input type="text" name="username" />
    <input type="password" name="pass" />
    <input type="submit" value="Log in" />
  </form>
</body>
</html>`

	reader := strings.NewReader(htmlStr)
	result, err := AnalyzePage(reader, "https://mydomain.com")
	if err != nil {
		t.Fatal(err)
	}

	if result.Title != "Login Test" {
		t.Errorf("Title = %q; want %q", result.Title, "Login Test")
	}
	if !result.HasLoginForm {
		t.Errorf("HasLoginForm = false; want true")
	}

	// With no anchors, all link counts should be zero
	if result.Links.Internal != 0 || result.Links.External != 0 || result.Links.Inaccessible != 0 {
		t.Errorf("Links = %+v; want all zeros", result.Links)
	}
}
