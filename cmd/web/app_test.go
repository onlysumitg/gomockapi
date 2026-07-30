package main

import (
	"net/http"
	"testing"
)

// Characterization test: proves the harness itself boots a working app
// (routes, embedded templates, session manager, BoltDB) before any of the
// behavioural tests lean on it.
func TestHarnessServesApp(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	t.Run("heartbeat", func(t *testing.T) {
		code, body := get(t, ts, "/ping")
		if code != http.StatusOK {
			t.Errorf("got status %d, want %d", code, http.StatusOK)
		}
		if body != "." {
			t.Errorf("got body %q, want %q", body, ".")
		}
	})

	t.Run("renders a template with a csrf token", func(t *testing.T) {
		code, _ := get(t, ts, "/user/login")
		if code != http.StatusOK {
			t.Errorf("got status %d, want %d", code, http.StatusOK)
		}
		if token := csrfToken(t, ts, "/user/login"); token == "" {
			t.Error("got empty csrf token")
		}
	})
}
