package main

import (
	"net/http"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// Verify is reached with a nil user whenever a verification link names an
// account that does not exist. Without the nil guard this panics, but only once
// a verification record exists: an empty bucket makes Verify return early,
// which is what hid the bug on a fresh database.
//
// This is the assertion that actually distinguishes the bug. The handler test
// below cannot: clientError has already written 400 by the time the
// dereference happens, so the status the client sees is 400 either way.
func TestVerifyWithNilUserDoesNotPanic(t *testing.T) {
	app := newTestApplication(t)

	user := &models.User{Name: "Real", Email: "nil@test.local", Password: "password123"}
	if err := app.users.Save(user, true); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := app.users.CreateNewVerification(user, app.users.GetVerificationTableName()); err != nil {
		t.Fatalf("create verification record: %v", err)
	}

	if app.users.Verify(nil, "whatever", app.users.GetVerificationTableName()) {
		t.Error("a nil user was reported as verified")
	}
}

// A verification link naming a user that does not exist must be rejected
// cleanly.
func TestVerifyUnknownUserIsRejected(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	existing := &models.User{Name: "Real", Email: "real@test.local", Password: "password123"}
	if err := app.users.Save(existing, true); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := app.users.CreateNewVerification(existing, app.users.GetVerificationTableName()); err != nil {
		t.Fatalf("create verification record: %v", err)
	}

	code, _ := get(t, ts, "/user/verify/NOSUCHUSER/whatever")
	if code != http.StatusBadRequest {
		t.Errorf("got status %d for an unknown user, want %d", code, http.StatusBadRequest)
	}
}

// The happy path still works: a correct link verifies the account.
func TestVerifyMarksUserVerified(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	user := &models.User{Name: "Real", Email: "verify@test.local", Password: "password123"}
	if err := app.users.Save(user, true); err != nil {
		t.Fatalf("create user: %v", err)
	}
	verificationID, err := app.users.CreateNewVerification(user, app.users.GetVerificationTableName())
	if err != nil {
		t.Fatalf("create verification record: %v", err)
	}

	if code, _ := get(t, ts, "/user/verify/"+user.ID+"/"+verificationID); code != http.StatusSeeOther {
		t.Fatalf("got status %d, want %d", code, http.StatusSeeOther)
	}

	stored, err := app.users.Get(user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !stored.HasVerified {
		t.Error("user is not marked verified after following a valid link")
	}
}
