package main

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// Request and response bodies may be a JSON array at the top level, not only
// an object. Their flat keys are "[0].key" style.
func TestTopLevelJsonArrays(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	createUser(t, app, "alice@test.local", true)

	sampleResponse := `[{"id":1,"name":"first"},{"id":2,"name":"second"}]`

	ep := &models.EndPoint{
		Name:                    "ARRAYS",
		Method:                  "POST",
		CollectionName:          "V1",
		ActualURL:               "http://example.com/x",
		SampleRequestType:       "JSON",
		SampleRequest:           `[{"key":"value","key2":"value2"}]`,
		SampleRequestHeaderType: "JSON",
		SampleRequestHeader:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID:                 "R_ARRAYS",
			Name:               "DEFAULT",
			HttpCode:           200,
			ResponseType:       "JSON",
			Response:           sampleResponse,
			ResponseHeaderType: "JSON",
			ResponseHeader:     "{}",
		}},
	}
	if _, err := app.endpoints.Save(ep, "alice@test.local"); err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}

	stored, err := app.endpoints.Get(ep.ID)
	if err != nil {
		t.Fatalf("reload endpoint: %v", err)
	}

	requestKeys := map[string]bool{}
	for _, p := range stored.RequestParams {
		requestKeys[p.Key] = true
	}
	for _, k := range []string{"[0].key", "[0].key2"} {
		if !requestKeys[k] {
			t.Errorf("request param %q missing, have %v", k, requestKeys)
		}
	}

	responseKeys := map[string]bool{}
	for _, p := range stored.ResponseMap[0].ResponseParams {
		responseKeys[p.Key] = true
	}
	for _, k := range []string{"[0].id", "[0].name", "[1].id", "[1].name"} {
		if !responseKeys[k] {
			t.Errorf("response param %q missing, have %v", k, responseKeys)
		}
	}

	resp, err := ts.Client().Post(ts.URL+"/api/V1/ARRAYS", "application/json",
		strings.NewReader(`[ { "key" :"value" , "key2" :"value2"}]`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d, body %s", resp.StatusCode, body)
	}

	var got, want any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("response is not JSON: %v: %s", err, body)
	}
	json.Unmarshal([]byte(sampleResponse), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response\n got  %s\n want %s", body, sampleResponse)
	}
}
