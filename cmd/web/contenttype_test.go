package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A proxied response carries no configured response type, because the body came
// from the upstream service rather than from a stored mock. Formatting that
// empty string into "application/%s" produced a bare "application/".
func TestWriteJSONorXMLContentType(t *testing.T) {
	tests := []struct {
		name          string
		responseType  string
		upstreamCType string
		want          string
	}{
		{"json", "JSON", "", "application/json"},
		{"xml", "XML", "", "application/xml"},
		{"text", "TEXT", "", "text/plain"},
		{"lowercase is normalised", "json", "", "application/json"},
		{"proxied response keeps upstream type", "", "text/html; charset=utf-8", "text/html; charset=utf-8"},
		{"proxied response with no upstream type", "", "", "application/octet-stream"},
	}

	app := &application{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			headers := http.Header{}
			if tt.upstreamCType != "" {
				headers.Set("Content-Type", tt.upstreamCType)
			}

			if err := app.writeJSONorXML(tt.responseType, rr, http.StatusOK, `{"a":1}`, headers); err != nil {
				t.Fatalf("writeJSONorXML: %v", err)
			}

			if got := rr.Header().Get("Content-Type"); got != tt.want {
				t.Errorf("Content-Type = %q, want %q", got, tt.want)
			}
		})
	}
}
