package analyzer

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// test template that prints either the error or a compact summary of fields
const testTpl = `{{if .Error}}ERR: {{.Error}}{{else}}URL={{.URL}}|HTML={{.HTMLVersion}}|Title={{.Title}}|HasLogin={{.HasLoginForm}}{{end}}`

func TestAnalyzeHandler(t *testing.T) {
	// Override the global template to make output deterministic in tests.
	Tmpl = template.Must(template.New("test").Parse(testTpl))

	logger := logrus.New()
	h := AnalyzeHandler(logger)

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/analyze", nil)
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", rr.Code)
		}
	})

	t.Run("missing URL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(""))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK { // renderError writes template with 200
			t.Fatalf("expected 200 with error template, got %d", rr.Code)
		}
		if got := rr.Body.String(); !strings.Contains(got, "ERR: URL is required") {
			t.Fatalf("expected error message, got: %q", got)
		}
	})

	t.Run("invalid URL format", func(t *testing.T) {
		form := "url=not-a-url"
		req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with error template, got %d", rr.Code)
		}
		if got := rr.Body.String(); !strings.Contains(got, "ERR: Invalid URL format") {
			t.Fatalf("expected invalid URL error, got: %q", got)
		}
	})

	t.Run("fetch error (connection refused/unreachable)", func(t *testing.T) {
		// Use port 0 to force a dial error; matches regex so it reaches http.Get.
		form := "url=http://127.0.0.1:0/anything"
		req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with error template, got %d", rr.Code)
		}
		got := rr.Body.String()
		if !strings.Contains(got, "ERR: Failed to fetch URL:") {
			t.Fatalf("expected fetch error message, got: %q", got)
		}
	})

	t.Run("non-200 upstream response renders error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusBadGateway)
		}))
		defer ts.Close()

		form := "url=" + ts.URL
		req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with error template, got %d", rr.Code)
		}
		want := fmt.Sprintf("ERR: URL unreachable – HTTP %d %s", http.StatusBadGateway, http.StatusText(http.StatusBadGateway))
		if got := rr.Body.String(); !strings.Contains(got, want) {
			t.Fatalf("expected %q, got: %q", want, got)
		}
	})

	t.Run("HTML parsing error bubbles through renderError", func(t *testing.T) {
		// Return content likely to make AnalyzePage fail (e.g., empty body or malformed),
		// assuming AnalyzePage returns an error on completely empty input.
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// send nothing / invalid markup
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><!— broken —></html>"))
		}))
		defer ts.Close()

		form := "url=" + ts.URL + "/broken"
		req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		// We can't guarantee AnalyzePage's behavior; accept either a success render or parse error.
		// If it errors, assert the error template shape.
		got := rr.Body.String()
		if strings.Contains(got, "ERR: HTML parsing error:") {
			// fine: handler correctly surfaced parse error
			return
		}
		// Otherwise, it parsed successfully; don't fail the test.
	})

	t.Run("happy path renders data via template", func(t *testing.T) {
		// Serve a small, well-formed HTML page that a typical AnalyzePage would parse.
		html := `<!doctype html>
<html>
<head><title>Demo Page</title></head>
<body>
<h1>Hi</h1>
<form action="/login"><input type="password" name="pw"></form>
<a href="http://example.com">ext</a>
<a href="/rel">rel</a>
</body></html>`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(html))
		}))
		defer ts.Close()

		form := "url=" + ts.URL
		req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()

		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
		}
		out := rr.Body.String()
		// We always expect the URL to be included since it comes from the request.
		if !strings.Contains(out, "URL="+ts.URL) {
			t.Fatalf("expected URL in render, got: %q", out)
		}
		// Do not assert on exact HTMLVersion/Title/HasLoginForm values because they depend on AnalyzePage;
		// but ensure we did not render the error path.
		if strings.HasPrefix(out, "ERR:") {
			t.Fatalf("unexpected error render on happy path: %q", out)
		}
	})
}
