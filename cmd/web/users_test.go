package main

import (
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// Email was lowercased after the record had already been marshalled, so the
// normalisation never reached the database. Lookups only worked because they
// all use EqualFold.
func TestSavedUserEmailIsStoredLowercase(t *testing.T) {
	app := newTestApplication(t)

	user := &models.User{Name: "Mixed", Email: "Mixed.Case@Test.Local", Password: "password123"}
	if err := app.users.Save(user, true); err != nil {
		t.Fatalf("create user: %v", err)
	}

	stored, err := app.users.Get(user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if stored.Email != "mixed.case@test.local" {
		t.Errorf("stored email = %q, want it normalised to lowercase", stored.Email)
	}
}

// Whatever the storage casing, authentication stays case-insensitive.
func TestAuthenticateIsCaseInsensitiveOnEmail(t *testing.T) {
	app := newTestApplication(t)

	const password = "password123"
	if err := app.users.Save(&models.User{Name: "Mixed", Email: "Mixed.Case@Test.Local", Password: password}, true); err != nil {
		t.Fatalf("create user: %v", err)
	}

	for _, email := range []string{"Mixed.Case@Test.Local", "mixed.case@test.local", "MIXED.CASE@TEST.LOCAL"} {
		if _, err := app.users.Authenticate(email, password); err != nil {
			t.Errorf("Authenticate(%q) failed: %v", email, err)
		}
	}
}
