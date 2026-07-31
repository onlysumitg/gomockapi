package main

import (
	"html"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// newTestApplication builds a fully wired *application backed by throwaway
// BoltDB files in t.TempDir().
//
// Two things to know before reusing this:
//   - BoltDB takes an exclusive file lock, so every test needs its own
//     directory. Sharing db/internal.db with a running server hangs.
//   - env.GetEnvVariable re-reads env/.env relative to the working directory on
//     every call, and the working directory under `go test` is this package.
//     Use t.Setenv rather than writing an .env file.
func newTestApplication(t *testing.T) *application {
	t.Helper()

	// The test server speaks plain HTTP, and env.UseHttps() defaults to true
	// when HTTPS is unset, which marks the session cookie Secure. A cookie jar
	// then refuses to send it over http:// and every login silently fails to
	// stick. Tests that care about the flag set it themselves before calling
	// this.
	if os.Getenv("HTTPS") == "" {
		t.Setenv("HTTPS", "false")
	}

	dir := t.TempDir()

	openDB := func(name string) *bolt.DB {
		db, err := bolt.Open(filepath.Join(dir, name), 0600, &bolt.Options{Timeout: 5 * time.Second})
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}

	// Deliberately goes through the production constructor so tests exercise the
	// real wiring rather than a parallel hand-rolled struct.
	app := baseAppConfig(parameters{host: "localhost", domain: "localhost"}, openDB("internal.db"), openDB("log.db"))

	return app
}

// newTestServer starts app.routes() on a random port with a cookie jar (needed
// for session + CSRF flows) and redirect following disabled so tests can assert
// on Location headers.
func newTestServer(t *testing.T, app *application) *httptest.Server {
	t.Helper()

	ts := httptest.NewServer(app.routes())
	t.Cleanup(ts.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	ts.Client().Jar = jar
	ts.Client().CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return ts
}

func get(t *testing.T, ts *httptest.Server, path string) (int, string) {
	t.Helper()

	rs, err := ts.Client().Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer rs.Body.Close()

	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatalf("GET %s: read body: %v", path, err)
	}

	return rs.StatusCode, string(body)
}

var csrfTokenRX = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

// csrfToken fetches a page and extracts its nosurf token. The matching cookie
// lands in the client's jar, so the token is only valid for that same client.
//
// The token is base64, and html/template escapes its "+" characters to "&#43;"
// in attribute context, so it has to be unescaped before being posted back.
// Skipping this fails only for tokens that happen to contain "+" — roughly two
// runs in three.
func csrfToken(t *testing.T, ts *httptest.Server, path string) string {
	t.Helper()

	_, body := get(t, ts, path)

	m := csrfTokenRX.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no csrf_token found on %s", path)
	}
	return html.UnescapeString(m[1])
}

func postForm(t *testing.T, ts *httptest.Server, path string, form url.Values) (int, string) {
	t.Helper()

	rs, err := ts.Client().Post(ts.URL+path, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer rs.Body.Close()

	return rs.StatusCode, rs.Header.Get("Location")
}

// closedPort returns a port with nothing listening on it, so that connecting to
// it fails immediately rather than hanging.
func closedPort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	return port
}
