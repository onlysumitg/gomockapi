package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// Signup fires the verification email on a goroutine *after* the response is
// written. If that goroutine terminates the process, a signup that looks
// successful to the client takes every mock endpoint down with it.
//
// This is an end-to-end test on purpose: it does not reference SendEmail's
// signature, so it compiles against the buggy code as well as the fix.
func TestSignupSurvivesUnreachableSMTP(t *testing.T) {
	app := newTestApplication(t)
	app.EmailServer.Host = "127.0.0.1"
	app.EmailServer.Port = closedPort(t)
	app.EmailServer.ConnectTimeout = 2 * time.Second

	ts := newTestServer(t, app)

	token := csrfToken(t, ts, "/user/signup")
	code, _ := postForm(t, ts, "/user/signup", url.Values{
		"csrf_token": {token},
		"name":       {"Test User"},
		"email":      {"harness@example.com"},
		"password":   {"password123"},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("signup: got status %d, want %d", code, http.StatusSeeOther)
	}

	// Keep polling while the send goroutine runs. Connecting to a closed local
	// port fails immediately, so this settles well inside the deadline.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if code, _ := get(t, ts, "/ping"); code != http.StatusOK {
			t.Fatalf("server stopped serving after signup: got status %d", code)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// The unit-level counterpart: a failed send is an error to be reported, not a
// reason to end the process.
func TestSendEmailReturnsErrorOnUnreachableSMTP(t *testing.T) {
	app := newTestApplication(t)
	app.EmailServer.Host = "127.0.0.1"
	app.EmailServer.Port = closedPort(t)
	app.EmailServer.ConnectTimeout = 2 * time.Second

	err := app.SendEmail(&models.EmailRequest{
		To:      []string{"nobody@example.com"},
		Subject: "test",
		Body:    "test",
	})
	if err == nil {
		t.Fatal("got nil error sending to an unreachable SMTP server, want an error")
	}
}

// A nil request is a no-op, not an error.
func TestSendEmailIgnoresNilRequest(t *testing.T) {
	app := newTestApplication(t)

	if err := app.SendEmail(nil); err != nil {
		t.Fatalf("got error %v, want nil", err)
	}
}
