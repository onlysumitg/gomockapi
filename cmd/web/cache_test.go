package main

import (
	"fmt"
	"sync"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

func seedEndPoint(t *testing.T, app *application, collection, name, method string) {
	t.Helper()

	ep := &models.EndPoint{
		Name:              name,
		Method:            method,
		CollectionName:    collection,
		ActualURL:         "http://example.com/x",
		SampleRequestType: "JSON",
		SampleRequest:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID:           fmt.Sprintf("R_%s", name),
			Name:         "DEFAULT",
			HttpCode:     200,
			ResponseType: "JSON",
			Response:     `{"ok":true}`,
		}},
	}

	if _, err := app.endpoints.Save(ep, "test@example.com"); err != nil {
		t.Fatalf("seed endpoint %s: %v", name, err)
	}
}

// The endpoint cache is read on every mock request and replaced wholesale on a
// miss or after invalidation. Run under -race: without synchronisation on both
// the map and the invalidation flag this reports a data race, and in production
// it can escalate to "concurrent map read and map write", which is a fatal
// error that no recover() can catch.
//
// Misses matter as much as hits here, since a miss is what triggers the
// rebuild.
func TestEndPointCacheConcurrentAccess(t *testing.T) {
	app := newTestApplication(t)

	seedEndPoint(t, app, "V1", "HIT", "GET")
	seedEndPoint(t, app, "V1", "OTHER", "POST")

	const readers = 8
	const iterations = 50

	var wg sync.WaitGroup

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// A cached endpoint must stay reachable even while another
				// goroutine is invalidating and rebuilding around it.
				if _, err := app.GetEndPoint("V1", "HIT", "GET"); err != nil {
					t.Errorf("cached endpoint went missing: %v", err)
					return
				}
				// Forces the rebuild path.
				_, _ = app.GetEndPoint("V1", fmt.Sprintf("nosuch-%d-%d", worker, j), "GET")
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < iterations; j++ {
			app.invalidateEndPointCache()
		}
	}()

	// The postman export ranges over the same shared map.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < iterations; j++ {
			app.EndpointsToPostmanCollection()
		}
	}()

	wg.Wait()
}
