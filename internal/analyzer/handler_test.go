package analyzer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAnalyzeHandler(t *testing.T) {
	tests := []struct {
		name       string
		formData   url.Values
		wantStatus int
	}{
		{"Missing URL", url.Values{}, http.StatusBadRequest},
		{"Invalid URL", url.Values{"url": {"invalid"}}, http.StatusBadRequest},
		// More tests later; for now, basic
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(tt.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			AnalyzeHandler(rr, req)
			if rr.Code != tt.wantStatus {
				t.Errorf("got %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}