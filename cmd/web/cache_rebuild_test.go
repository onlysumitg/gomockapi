package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// A miss used to rebuild the entire cache from BoltDB: every endpoint plus a
// full bucket scan per endpoint for request params, response params and
// condition groups. /api/* is unauthenticated and unthrottled by default, so a
// loop of requests for a name that does not exist was an amplification lever.
//
// Counts read transactions rather than elapsed time: bbolt's TxN is exact,
// where a timing assertion would be flaky in CI.
func TestRepeatedMissesDoNotRebuildCache(t *testing.T) {
	app := newTestApplication(t)
	seedEndPoint(t, app, "V1", "HIT", "GET")

	// Warm the cache so the initial build is not counted.
	if _, err := app.GetEndPoint("V1", "HIT", "GET"); err != nil {
		t.Fatalf("seeded endpoint not found: %v", err)
	}

	before := app.DB.Stats().TxN

	const misses = 50
	for i := 0; i < misses; i++ {
		if _, err := app.GetEndPoint("V1", "nosuchendpoint", "GET"); err == nil {
			t.Fatal("expected a miss for an endpoint that was never created")
		}
	}

	if got := app.DB.Stats().TxN - before; got != 0 {
		t.Errorf("%d misses triggered %d read transactions, want 0: a cache that has not been invalidated already knows the endpoint is absent", misses, got)
	}
}

// The flip side of the optimisation above: once a miss is trusted, anything
// that creates an endpoint must invalidate the cache, or the new endpoint stays
// invisible until the process restarts.
func TestEndPointCreatedAfterMissBecomesVisible(t *testing.T) {
	app := newTestApplication(t)
	seedEndPoint(t, app, "V1", "FIRST", "GET")

	if _, err := app.GetEndPoint("V1", "FIRST", "GET"); err != nil {
		t.Fatalf("seeded endpoint not found: %v", err)
	}
	if _, err := app.GetEndPoint("V1", "SECOND", "GET"); err == nil {
		t.Fatal("expected a miss before the second endpoint exists")
	}

	seedEndPoint(t, app, "V1", "SECOND", "GET")
	app.invalidateEndPointCache()

	if _, err := app.GetEndPoint("V1", "SECOND", "GET"); err != nil {
		t.Errorf("endpoint created after a cached miss is still invisible: %v", err)
	}
}

// End-to-end guard on the thing the app exists to do, and the regression the
// cache changes above would most plausibly cause: a real endpoint must still be
// served over HTTP.
func TestSeededEndPointIsServedOverHTTP(t *testing.T) {
	app := newTestApplication(t)
	seedEndPoint(t, app, "V1", "HIT", "GET")

	ts := newTestServer(t, app)

	code, body := get(t, ts, "/api/V1/HIT")
	if code != http.StatusOK {
		t.Fatalf("got status %d, want %d (body %q)", code, http.StatusOK, body)
	}
	if !strings.Contains(body, `"ok"`) {
		t.Errorf("got body %q, want it to contain the configured response", body)
	}
}

const minimalPostmanCollection = `{
  "info": {
    "name": "ImportedColl",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "greet",
      "request": {
        "method": "GET",
        "header": [],
        "url": { "raw": "http://example.com/greet" }
      }
    }
  ]
}`

// Importing a Postman collection creates endpoints straight through the model,
// bypassing the endpoint handlers. It has to invalidate the cache too, or an
// imported endpoint 404s until something unrelated happens to invalidate.
func TestImportedEndPointIsServable(t *testing.T) {
	app := newTestApplication(t)

	// Prime the cache so the import lands against a cache believed to be fresh.
	if _, err := app.GetEndPoint("ImportedColl", "greet", "GET"); err == nil {
		t.Fatal("expected a miss before importing")
	}

	// ReadPostmantJson deletes the file it is given, so write a throwaway copy.
	path := filepath.Join(t.TempDir(), "collection.json")
	if err := os.WriteFile(path, []byte(minimalPostmanCollection), 0600); err != nil {
		t.Fatalf("write collection: %v", err)
	}

	messages := app.ReadPostmantJson(path, &models.User{Email: "importer@example.com"})

	if _, err := app.GetEndPoint("ImportedColl", "greet", "GET"); err != nil {
		t.Errorf("imported endpoint is not servable: %v\nimport log: %v", err, messages)
	}
}
