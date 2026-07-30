package main

import (
	"strings"
	"testing"
)

// The session cookie authenticates every admin request, so it must not travel
// over plaintext when the server is serving HTTPS. It stays unmarked when it is
// not, otherwise local HTTP development could never log in.
func TestSessionCookieSecureFollowsHTTPS(t *testing.T) {
	tests := []struct {
		name       string
		https      string
		wantSecure bool
	}{
		{"https enabled", "true", true},
		{"https disabled", "false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set before building the app: getSessionManager reads this once,
			// during baseAppConfig.
			t.Setenv("HTTPS", tt.https)

			app := newTestApplication(t)
			ts := newTestServer(t, app)

			rs, err := ts.Client().Get(ts.URL + "/user/signup")
			if err != nil {
				t.Fatalf("GET /user/signup: %v", err)
			}
			defer rs.Body.Close()

			var sessionCookie string
			for _, c := range rs.Header.Values("Set-Cookie") {
				if strings.HasPrefix(c, "session=") {
					sessionCookie = c
				}
			}
			if sessionCookie == "" {
				t.Fatalf("no session cookie in %v", rs.Header.Values("Set-Cookie"))
			}

			gotSecure := strings.Contains(sessionCookie, "Secure")
			if gotSecure != tt.wantSecure {
				t.Errorf("session cookie Secure = %t, want %t (%q)", gotSecure, tt.wantSecure, sessionCookie)
			}

			// HttpOnly and SameSite should hold either way.
			if !strings.Contains(sessionCookie, "HttpOnly") {
				t.Errorf("session cookie is not HttpOnly (%q)", sessionCookie)
			}
			if !strings.Contains(sessionCookie, "SameSite=Lax") {
				t.Errorf("session cookie is not SameSite=Lax (%q)", sessionCookie)
			}
		})
	}
}
