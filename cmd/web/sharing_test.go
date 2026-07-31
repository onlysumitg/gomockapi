package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// createUser saves a verified user and returns it.
func createUser(t *testing.T, app *application, email string, superuser bool) *models.User {
	t.Helper()

	user := &models.User{
		Name:        email,
		Email:       email,
		Password:    "password123",
		HasVerified: true,
		IsSuperUser: superuser,
	}
	if err := app.users.Save(user, true); err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}

	return user
}

func login(t *testing.T, ts *httptest.Server, email string) {
	t.Helper()

	code, _ := postForm(t, ts, "/user/login", url.Values{
		"csrf_token": {csrfToken(t, ts, "/user/login")},
		"email":      {email},
		"password":   {"password123"},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("login %s: got status %d, want %d", email, code, http.StatusSeeOther)
	}
	if code, _ := get(t, ts, "/endpoints"); code != http.StatusOK {
		t.Fatalf("login %s did not establish a session: /endpoints returned %d", email, code)
	}
}

// seedOwnedEndPoint creates an endpoint attributed to owner and registers them
// as its owner, mirroring what EndPointAddPost does.
func seedOwnedEndPoint(t *testing.T, app *application, owner *models.User, name string) *models.EndPoint {
	t.Helper()

	ep := &models.EndPoint{
		Name:              name,
		Method:            "GET",
		CollectionName:    "V1",
		ActualURL:         "http://example.com/x",
		SampleRequestType: "JSON",
		SampleRequest:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID: "R_" + name, Name: "DEFAULT", HttpCode: 200, ResponseType: "JSON", Response: `{"ok":true}`,
		}},
	}

	id, err := app.endpoints.Save(ep, owner.Email)
	if err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}

	owner.AssignOwnedEndPoint(id)
	if err := app.users.Save(owner, false); err != nil {
		t.Fatalf("assign owner: %v", err)
	}

	return ep
}

func shareWith(t *testing.T, ts *httptest.Server, endpointID, email string) (int, string) {
	t.Helper()

	return postForm(t, ts, "/endpoints/addowners/"+endpointID, url.Values{
		"csrf_token": {csrfToken(t, ts, "/endpoints/owners/"+endpointID)},
		"email":      {email},
	})
}

// The point of the feature: someone an endpoint is shared with can work on it.
func TestSharedEndPointIsUsableByRecipient(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	alice := createUser(t, app, "alice@test.local", false)
	createUser(t, app, "bob@test.local", false)
	endpoint := seedOwnedEndPoint(t, app, alice, "SHARED")

	login(t, ts, alice.Email)
	if code, _ := shareWith(t, ts, endpoint.ID, "bob@test.local"); code != http.StatusSeeOther {
		t.Fatalf("share: got status %d, want %d", code, http.StatusSeeOther)
	}

	login(t, ts, "bob@test.local")
	if code, _ := get(t, ts, "/endpoints/"+endpoint.ID); code != http.StatusOK {
		t.Errorf("recipient cannot view the shared endpoint: got status %d", code)
	}
	if code, _ := get(t, ts, "/endpoints/update/"+endpoint.ID); code != http.StatusOK {
		t.Errorf("recipient cannot edit the shared endpoint: got status %d", code)
	}
}

// Sharing grants access to that endpoint only.
func TestUnsharedEndPointStaysPrivate(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	alice := createUser(t, app, "alice@test.local", false)
	createUser(t, app, "bob@test.local", false)
	endpoint := seedOwnedEndPoint(t, app, alice, "PRIVATE")

	login(t, ts, "bob@test.local")
	if code, _ := get(t, ts, "/endpoints/"+endpoint.ID); code == http.StatusOK {
		t.Error("a user who was not shared the endpoint can view it")
	}
}

// Co-owners get the endpoint, not the guest list: only the creator and
// superadmins decide who has access, so a co-owner cannot lock the creator out.
func TestCoOwnerCannotManageOwners(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	alice := createUser(t, app, "alice@test.local", false)
	createUser(t, app, "bob@test.local", false)
	createUser(t, app, "eve@test.local", false)
	endpoint := seedOwnedEndPoint(t, app, alice, "GUESTLIST")

	login(t, ts, alice.Email)
	if code, _ := shareWith(t, ts, endpoint.ID, "bob@test.local"); code != http.StatusSeeOther {
		t.Fatalf("share with bob: got status %d", code)
	}

	login(t, ts, "bob@test.local")
	if code, _ := get(t, ts, "/endpoints/owners/"+endpoint.ID); code == http.StatusOK {
		t.Error("a co-owner can reach the owners page")
	}

	// Bob cannot hand access to Eve. He also cannot read the owners page for a
	// CSRF token, so post with one taken from a page he may see.
	code, _ := postForm(t, ts, "/endpoints/addowners/"+endpoint.ID, url.Values{
		"csrf_token": {csrfToken(t, ts, "/endpoints/update/"+endpoint.ID)},
		"email":      {"eve@test.local"},
	})
	if code == http.StatusSeeOther {
		t.Error("a co-owner shared the endpoint onwards")
	}

	eve, err := app.users.GetByEmail("eve@test.local")
	if err != nil {
		t.Fatalf("reload eve: %v", err)
	}
	if eve.HasOwnedEndPoint(endpoint.ID) {
		t.Error("eve gained access via a co-owner")
	}
}

func TestCreatorCannotBeRemoved(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	alice := createUser(t, app, "alice@test.local", false)
	endpoint := seedOwnedEndPoint(t, app, alice, "CREATOR")

	login(t, ts, alice.Email)

	code, _ := postForm(t, ts, "/endpoints/removeowner/"+endpoint.ID, url.Values{
		"csrf_token": {csrfToken(t, ts, "/endpoints/owners/"+endpoint.ID)},
		"email":      {alice.Email},
	})
	if code == http.StatusSeeOther {
		t.Fatal("the creator was removed from their own endpoint")
	}

	reloaded, err := app.users.Get(alice.ID)
	if err != nil {
		t.Fatalf("reload alice: %v", err)
	}
	if !reloaded.HasOwnedEndPoint(endpoint.ID) {
		t.Error("the creator lost access to their own endpoint")
	}
}

func TestRemoveOwnerWithdrawsAccess(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	alice := createUser(t, app, "alice@test.local", false)
	createUser(t, app, "bob@test.local", false)
	endpoint := seedOwnedEndPoint(t, app, alice, "REVOKE")

	login(t, ts, alice.Email)
	if code, _ := shareWith(t, ts, endpoint.ID, "bob@test.local"); code != http.StatusSeeOther {
		t.Fatalf("share: got status %d", code)
	}

	code, _ := postForm(t, ts, "/endpoints/removeowner/"+endpoint.ID, url.Values{
		"csrf_token": {csrfToken(t, ts, "/endpoints/owners/"+endpoint.ID)},
		"email":      {"bob@test.local"},
	})
	if code != http.StatusSeeOther {
		t.Fatalf("remove: got status %d, want %d", code, http.StatusSeeOther)
	}

	login(t, ts, "bob@test.local")
	if code, _ := get(t, ts, "/endpoints/"+endpoint.ID); code == http.StatusOK {
		t.Error("a removed owner can still view the endpoint")
	}
}

func TestShareRejectsUnknownAndDuplicateEmails(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	alice := createUser(t, app, "alice@test.local", false)
	createUser(t, app, "bob@test.local", false)
	endpoint := seedOwnedEndPoint(t, app, alice, "VALIDATION")

	login(t, ts, alice.Email)

	if code, _ := shareWith(t, ts, endpoint.ID, "nobody@test.local"); code != http.StatusUnprocessableEntity {
		t.Errorf("sharing with an unknown email: got status %d, want %d", code, http.StatusUnprocessableEntity)
	}

	if code, _ := shareWith(t, ts, endpoint.ID, "bob@test.local"); code != http.StatusSeeOther {
		t.Fatalf("first share: got status %d", code)
	}
	if code, _ := shareWith(t, ts, endpoint.ID, "bob@test.local"); code != http.StatusUnprocessableEntity {
		t.Errorf("sharing twice: got status %d, want %d", code, http.StatusUnprocessableEntity)
	}
}

// Quota counts what a user created. If it counted what they own, anyone could
// exhaust anyone else's allowance by sharing endpoints at them.
func TestSharedEndPointsDoNotConsumeRecipientQuota(t *testing.T) {
	app := newTestApplication(t)

	alice := createUser(t, app, "alice@test.local", false)

	bob := createUser(t, app, "bob@test.local", false)
	bob.MaxAllowedEndpoints = 2
	if err := app.users.Save(bob, false); err != nil {
		t.Fatalf("set quota: %v", err)
	}

	// Alice shares more endpoints than Bob's whole allowance.
	for _, name := range []string{"ONE", "TWO", "THREE"} {
		endpoint := seedOwnedEndPoint(t, app, alice, name)
		bob.AssignOwnedEndPoint(endpoint.ID)
	}
	if err := app.users.Save(bob, false); err != nil {
		t.Fatalf("share endpoints: %v", err)
	}

	if got := app.endpoints.CountCreatedBy(bob.Email); got != 0 {
		t.Errorf("CountCreatedBy(bob) = %d, want 0: bob created none of them", got)
	}
	if got := app.endpoints.CountCreatedBy(alice.Email); got != 3 {
		t.Errorf("CountCreatedBy(alice) = %d, want 3", got)
	}
}
