package main

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

func TestSafeRedirectTarget(t *testing.T) {
	const fallback = "/endpoints"

	tests := []struct {
		name string
		next string
		want string
	}{
		{"empty falls back", "", fallback},
		{"relative path is kept", "/endpoints/add", "/endpoints/add"},
		{"query string is kept", "/endpoints?cid=1", "/endpoints?cid=1"},
		{"absolute http url", "http://evil.com", fallback},
		{"absolute https url", "https://evil.com/path", fallback},
		{"protocol relative", "//evil.com", fallback},
		{"backslash protocol relative", `/\evil.com`, fallback},
		{"double backslash", `\\evil.com`, fallback},
		{"scheme relative with creds", "https://user@evil.com", fallback},
		{"whitespace padded absolute", "  https://evil.com  ", fallback},
		{"javascript scheme", "javascript:alert(1)", fallback},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeRedirectTarget(tt.next, fallback); got != tt.want {
				t.Errorf("safeRedirectTarget(%q) = %q, want %q", tt.next, got, tt.want)
			}
		})
	}
}

// The redirect happens after credentials are accepted, so an unchecked ?next=
// sends an authenticated user straight to an attacker's page.
func TestLoginDoesNotRedirectOffSite(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	const email = "redirect@test.local"
	const password = "password123"

	if err := app.users.Save(&models.User{
		Name:        "Redirect",
		Email:       email,
		Password:    password,
		HasVerified: true,
	}, true); err != nil {
		t.Fatalf("create user: %v", err)
	}

	tests := []struct {
		name string
		next string
		want string
	}{
		{"offsite absolute", "https://evil.com", "/endpoints"},
		{"offsite protocol relative", "//evil.com", "/endpoints"},
		{"offsite backslash", `/\evil.com`, "/endpoints"},
		{"in-app path is honoured", "/endpoints/add", "/endpoints/add"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/user/login?next=" + url.QueryEscape(tt.next)

			code, location := postForm(t, ts, path, url.Values{
				"csrf_token": {csrfToken(t, ts, "/user/login")},
				"email":      {email},
				"password":   {password},
			})
			if code != http.StatusSeeOther {
				t.Fatalf("got status %d, want %d", code, http.StatusSeeOther)
			}
			if location != tt.want {
				t.Errorf("redirected to %q after login, want %q", location, tt.want)
			}
		})
	}
}
