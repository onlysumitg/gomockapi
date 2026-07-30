package httputils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The proxy forwards the caller's headers upstream. Overwriting Content-Type
// unconditionally told the upstream service that an XML body was JSON.
func TestHttpProcessRequestPreservesContentType(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
		want     string
	}{
		{"forwarded type is kept", "application/xml", "application/xml"},
		{"absent type defaults to json", "", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.Header.Get("Content-Type")
			}))
			defer upstream.Close()

			header := http.Header{}
			if tt.incoming != "" {
				header.Set("Content-Type", tt.incoming)
			}

			if res := HttpPOST(upstream.URL, header, []byte("<a/>")); res.Err != nil {
				t.Fatalf("HttpPOST: %v", res.Err)
			}

			if seen != tt.want {
				t.Errorf("upstream saw Content-Type %q, want %q", seen, tt.want)
			}
		})
	}
}

// A caller that forwards no headers at all must not panic on a nil map.
func TestHttpProcessRequestHandlesNilHeader(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer upstream.Close()

	if res := HttpGET(upstream.URL, nil); res.Err != nil {
		t.Fatalf("HttpGET with nil header: %v", res.Err)
	}
}
