package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// loginAsSuperUser creates a verified superuser and authenticates the test
// server's client as that user.
func loginAsSuperUser(t *testing.T, app *application, ts *httptest.Server) {
	t.Helper()

	const email = "admin@test.local"
	const password = "password123"

	user := &models.User{
		Name:        "Admin",
		Email:       email,
		Password:    password,
		IsSuperUser: true,
		IsStaff:     true,
		HasVerified: true,
	}
	if err := app.users.Save(user, true); err != nil {
		t.Fatalf("create superuser: %v", err)
	}

	code, _ := postForm(t, ts, "/user/login", url.Values{
		"csrf_token": {csrfToken(t, ts, "/user/login")},
		"email":      {email},
		"password":   {password},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("login: got status %d, want %d", code, http.StatusSeeOther)
	}

	// A 303 from the login POST is not proof of anything: RequireAuthentication
	// answers with 303 too, so a dropped session cookie looks identical at
	// every later call site. Confirm a protected page actually renders.
	if code, _ := get(t, ts, "/endpoints"); code != http.StatusOK {
		t.Fatalf("login did not establish a session: GET /endpoints returned %d", code)
	}
}

// API call logs are written to and read from LogDB (db/log_YYYYMMDD.db), so
// that is where clearing them has to happen. Clearing internal.db instead
// silently does nothing, which is why db/log.db grew to 207MB.
//
// Goes through the HTTP handler rather than calling ClearLogs directly: the
// defect was which database the handler passed, so a model-level test would
// not have caught it.
func TestClearApiLogsClearsTheLogDatabase(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	loginAsSuperUser(t, app, ts)

	const correlationID = "CORRELATION-1"
	apiCall := &models.ApiCall{
		ID:  correlationID,
		DB:  app.LogDB,
		Log: []string{"first entry", "second entry"},
	}
	apiCall.SaveLogs()

	if got := len(models.GetLogs(app.LogDB, correlationID)); got != 2 {
		t.Fatalf("precondition: got %d stored log entries, want 2", got)
	}

	if code, _ := get(t, ts, "/apilogs/clear"); code != http.StatusSeeOther {
		t.Fatalf("clear: got status %d, want %d", code, http.StatusSeeOther)
	}

	if got := len(models.GetLogs(app.LogDB, correlationID)); got != 0 {
		t.Errorf("got %d log entries after clearing, want 0", got)
	}
}

// The same clear also resets the per-endpoint call log, which lives in
// internal.db. Both databases have to be handled, not one or the other.
func TestClearApiLogsResetsEndPointCallLog(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	loginAsSuperUser(t, app, ts)

	seedEndPoint(t, app, "V1", "LOGGED", "GET")

	endPoints := app.endpoints.List()
	if len(endPoints) != 1 {
		t.Fatalf("precondition: got %d endpoints, want 1", len(endPoints))
	}
	endPoint := endPoints[0]
	endPoint.EndPointCallLog = append(endPoint.EndPointCallLog, models.EndPointCallLog{CorellationID: "CORRELATION-1"})
	if _, err := app.endpoints.Save(endPoint, "test@example.com"); err != nil {
		t.Fatalf("seed call log: %v", err)
	}

	if code, _ := get(t, ts, "/apilogs/clear"); code != http.StatusSeeOther {
		t.Fatalf("clear: got status %d, want %d", code, http.StatusSeeOther)
	}

	stored, err := app.endpoints.Get(endPoint.ID)
	if err != nil {
		t.Fatalf("reload endpoint: %v", err)
	}
	if got := len(stored.EndPointCallLog); got != 0 {
		t.Errorf("got %d endpoint call log entries after clearing, want 0", got)
	}
}
