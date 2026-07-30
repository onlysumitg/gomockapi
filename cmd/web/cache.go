package main

import (
	"fmt"
	"strings"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// invalidateEndPointCache marks the cache stale so the next lookup rebuilds it.
func (app *application) invalidateEndPointCache() {
	app.endPointMutex.Lock()
	app.invalidEndPointCache = true
	app.endPointMutex.Unlock()
}

// GetEndPoint resolves a mock endpoint by collection, name and method,
// rebuilding the cache on a miss or after invalidation.
//
// endPointCache and invalidEndPointCache are shared by every in-flight request,
// so both are read and written under endPointMutex. An unsynchronised read
// racing the wholesale map replacement below can trip Go's "concurrent map read
// and map write" fatal error, which no recover() can catch.
func (app *application) GetEndPoint(collection, endpointname, httpmethod string) (*models.EndPoint, error) {

	endPointKey := fmt.Sprintf("%s_%s_%s", strings.ToLower(collection), strings.ToLower(endpointname), strings.ToLower(httpmethod))

	app.endPointMutex.RLock()
	endPoint, found := app.endPointCache[endPointKey]
	stale := app.invalidEndPointCache
	app.endPointMutex.RUnlock()

	if !stale {
		// The cache mirrors the database until something invalidates it, so a
		// miss here is a real miss and rebuilding would not turn it into a hit.
		// Rebuilding anyway meant every request for an unknown name re-read
		// every endpoint and all of its params from BoltDB.
		if !found {
			return nil, endPointNotFoundError(collection, endpointname, httpmethod)
		}
		return endPoint, nil
	}

	app.endPointMutex.Lock()
	defer app.endPointMutex.Unlock()

	// Another goroutine may have rebuilt the cache while we waited for the lock,
	// in which case there is nothing left to do.
	if app.invalidEndPointCache {
		app.endPointCache = app.endpoints.BuildEndPointCache(app.maxAllowedEndPoints)
		app.invalidEndPointCache = false
	}

	endPoint, found = app.endPointCache[endPointKey]
	if !found {
		return nil, endPointNotFoundError(collection, endpointname, httpmethod)
	}

	return endPoint, nil
}

func endPointNotFoundError(collection, endpointname, httpmethod string) error {
	return fmt.Errorf("not found: %s %s %s", collection, endpointname, httpmethod)
}

// cachedEndPoints returns a snapshot of the currently cached endpoints, safe to
// range over after the lock is released. Callers must treat the endpoints as
// read-only: the pointers are shared with every other request in flight.
func (app *application) cachedEndPoints() []*models.EndPoint {
	app.endPointMutex.RLock()
	defer app.endPointMutex.RUnlock()

	endPoints := make([]*models.EndPoint, 0, len(app.endPointCache))
	for _, endPoint := range app.endPointCache {
		endPoints = append(endPoints, endPoint)
	}

	return endPoints
}
