package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/utils/xmlutils"
)

func TestIsCacheable(t *testing.T) {
	tests := []struct {
		name string
		ep   models.EndPoint
		want bool
	}{
		{
			name: "no conditions, no dynamic defaults",
			ep: models.EndPoint{
				ConditionGroups: []*models.ConditionGroup{},
				ResponseMap: []*models.EndPointResponse{{
					ResponseParams: []*models.EndPointResponseParam{
						{OverrideValue: "literal"},
					},
				}},
			},
			want: true,
		},
		{
			name: "condition group with literal assign",
			ep: models.EndPoint{
				ConditionGroups: []*models.ConditionGroup{{
					ConditionGroupParameters: []*models.ConditionGroupParameter{
						{AssgineValue: "42"},
					},
				}},
				ResponseMap: []*models.EndPointResponse{{
					ResponseParams: []*models.EndPointResponseParam{},
				}},
			},
			want: true,
		},
		{
			name: "condition group with RANDOM assign",
			ep: models.EndPoint{
				ConditionGroups: []*models.ConditionGroup{{
					ConditionGroupParameters: []*models.ConditionGroupParameter{
						{AssgineValue: "*RANDOM:NAME"},
					},
				}},
				ResponseMap: []*models.EndPointResponse{{
					ResponseParams: []*models.EndPointResponseParam{},
				}},
			},
			want: false,
		},
		{
			name: "condition group with REQUEST assign",
			ep: models.EndPoint{
				ConditionGroups: []*models.ConditionGroup{{
					ConditionGroupParameters: []*models.ConditionGroupParameter{
						{AssgineValue: "REQUEST[STRING]:some_field"},
					},
				}},
				ResponseMap: []*models.EndPointResponse{{
					ResponseParams: []*models.EndPointResponseParam{},
				}},
			},
			want: false,
		},
		{
			name: "condition group with proxy",
			ep: models.EndPoint{
				ConditionGroups: []*models.ConditionGroup{{
					CallActualEndPoint: true,
				}},
				ResponseMap: []*models.EndPointResponse{{
					ResponseParams: []*models.EndPointResponseParam{},
				}},
			},
			want: false,
		},
		{
			name: "default response param with RANDOM override",
			ep: models.EndPoint{
				ConditionGroups: []*models.ConditionGroup{},
				ResponseMap: []*models.EndPointResponse{{
					ResponseParams: []*models.EndPointResponseParam{
						{OverrideValue: "*RANDOM:EMAIL"},
					},
				}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ep.IsCacheable()
			if got != tt.want {
				t.Errorf("IsCacheable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildResponseCacheKey(t *testing.T) {
	flatMap := map[string]xmlutils.ValueDatatype{
		"last_name":  {Value: "Smith", DataType: "STRING"},
		"first_name": {Value: "John", DataType: "STRING"},
		"age":        {Value: "30", DataType: "STRING"},
	}

	// Only last_name and first_name are condition keys; age is ignored.
	key1 := buildResponseCacheKey("v1_users_get", []string{"last_name", "first_name"}, flatMap)
	key2 := buildResponseCacheKey("v1_users_get", []string{"first_name", "last_name"}, flatMap)

	// Order of conditionKeys should not matter (sorted internally).
	if key1 != key2 {
		t.Errorf("cache keys differ for same values in different order:\n  %s\n  %s", key1, key2)
	}

	// Different values produce different keys.
	flatMap2 := map[string]xmlutils.ValueDatatype{
		"last_name":  {Value: "Jones", DataType: "STRING"},
		"first_name": {Value: "John", DataType: "STRING"},
	}
	key3 := buildResponseCacheKey("v1_users_get", []string{"last_name", "first_name"}, flatMap2)
	if key1 == key3 {
		t.Error("cache keys should differ for different request values")
	}
}

func TestResponseCacheHitSkipsBuildResponse(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	ep := &models.EndPoint{
		Name:                    "CACHEABLE",
		Method:                  "GET",
		CollectionName:          "V1",
		ActualURL:               "http://example.com/x",
		SampleRequestType:       "JSON",
		SampleRequest:           `{"last_name":"test"}`,
		SampleRequestHeaderType: "JSON",
		SampleRequestHeader:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID:                 "R_CACHEABLE",
			Name:               "DEFAULT",
			HttpCode:           200,
			ResponseType:       "JSON",
			Response:           `{"greeting":"hello"}`,
			ResponseHeaderType: "JSON",
			ResponseHeader:     "{}",
		}},
	}

	if _, err := app.endpoints.Save(ep, "test@example.com"); err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}
	app.invalidateEndPointCache()

	// First call: cache miss, populates cache.
	code1, body1 := get(t, ts, "/api/V1/CACHEABLE?last_name=Smith")
	if code1 != http.StatusOK {
		t.Fatalf("first call: got status %d, want %d", code1, http.StatusOK)
	}

	// Second call: same params, should hit cache and return same result.
	code2, body2 := get(t, ts, "/api/V1/CACHEABLE?last_name=Smith")
	if code2 != http.StatusOK {
		t.Fatalf("second call: got status %d, want %d", code2, http.StatusOK)
	}
	if body1 != body2 {
		t.Errorf("cache hit returned different body:\n  first:  %s\n  second: %s", body1, body2)
	}

	// Third call: different param value, should be a cache miss.
	code3, body3 := get(t, ts, "/api/V1/CACHEABLE?last_name=Jones")
	if code3 != http.StatusOK {
		t.Fatalf("third call: got status %d, want %d", code3, http.StatusOK)
	}
	// Body should still be valid (same default response, no conditions).
	if body3 == "" {
		t.Error("third call returned empty body")
	}
}

func TestResponseCacheInvalidatedWithEndpointCache(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	ep := &models.EndPoint{
		Name:                    "INVAL",
		Method:                  "GET",
		CollectionName:          "V1",
		ActualURL:               "http://example.com/x",
		SampleRequestType:       "JSON",
		SampleRequest:           `{"x":"1"}`,
		SampleRequestHeaderType: "JSON",
		SampleRequestHeader:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID:                 "R_INVAL",
			Name:               "DEFAULT",
			HttpCode:           200,
			ResponseType:       "JSON",
			Response:           `{"v":"old"}`,
			ResponseHeaderType: "JSON",
			ResponseHeader:     "{}",
		}},
	}

	if _, err := app.endpoints.Save(ep, "test@example.com"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	app.invalidateEndPointCache()

	// Populate cache.
	code1, body1 := get(t, ts, "/api/V1/INVAL?x=1")
	if code1 != http.StatusOK {
		t.Fatalf("got %d", code1)
	}

	// Update the response body.
	stored, err := app.endpoints.Get(ep.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	stored.ResponseMap[0].Response = `{"v":"new"}`
	if _, err := app.endpoints.Save(stored, "test@example.com"); err != nil {
		t.Fatalf("update: %v", err)
	}
	app.invalidateEndPointCache()

	// After invalidation, should get new response.
	code2, body2 := get(t, ts, "/api/V1/INVAL?x=1")
	if code2 != http.StatusOK {
		t.Fatalf("got %d", code2)
	}
	if body1 == body2 {
		t.Error("response cache was not cleared on invalidation: got same body after endpoint update")
	}
}

func TestNonCacheableEndpointSkipsCache(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	ep := &models.EndPoint{
		Name:                    "NOCACHE",
		Method:                  "GET",
		CollectionName:          "V1",
		ActualURL:               "http://example.com/x",
		SampleRequestType:       "JSON",
		SampleRequest:           `{"x":"1"}`,
		SampleRequestHeaderType: "JSON",
		SampleRequestHeader:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID:                 "R_NOCACHE",
			Name:               "DEFAULT",
			HttpCode:           200,
			ResponseType:       "JSON",
			Response:           `{"name":"placeholder"}`,
			ResponseHeaderType: "JSON",
			ResponseHeader:     "{}",
		}},
	}

	if _, err := app.endpoints.Save(ep, "test@example.com"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	app.invalidateEndPointCache()

	// Set an override with *RANDOM on the response param to make it non-cacheable.
	stored, err := app.endpoints.Get(ep.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	for _, r := range stored.ResponseMap {
		for _, rp := range r.ResponseParams {
			if rp.Key == "name" {
				rp.OverrideValue = "*RANDOM:NAME"
				app.responseParams.Update(rp, false)
			}
		}
	}
	app.invalidateEndPointCache()

	// Each call should potentially differ (random).
	// At minimum, the endpoint should respond OK.
	code, _ := get(t, ts, "/api/V1/NOCACHE?x=1")
	if code != http.StatusOK {
		t.Fatalf("got %d, want 200", code)
	}

	// Verify no cache entry was stored.
	epKey := fmt.Sprintf("v1_nocache_get")
	cacheKey := buildResponseCacheKey(epKey, []string{"x"}, map[string]xmlutils.ValueDatatype{
		"x": {Value: "1", DataType: "STRING"},
	})
	if _, ok := app.getResponseCache(cacheKey); ok {
		t.Error("non-cacheable endpoint should not have a cache entry")
	}
}
