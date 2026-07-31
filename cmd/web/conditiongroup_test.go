package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/onlysumitg/GoMockAPI/internal/models"
)

// seedEndPointWithNumericResponse creates an endpoint whose response contains a
// numeric field, so its response parameter is typed FLOAT64 rather than STRING.
func seedEndPointWithNumericResponse(t *testing.T, app *application, name string) *models.EndPoint {
	t.Helper()

	ep := &models.EndPoint{
		Name:                    name,
		Method:                  "GET",
		CollectionName:          "V1",
		ActualURL:               "http://example.com/x",
		SampleRequestType:       "JSON",
		SampleRequest:           `{"count":1}`,
		SampleRequestHeaderType: "JSON",
		SampleRequestHeader:     "{}",
		ResponseMap: []*models.EndPointResponse{{
			ID:                 "R_" + name,
			Name:               "DEFAULT",
			HttpCode:           200,
			ResponseType:       "JSON",
			Response:           `{"count":1}`,
			ResponseHeaderType: "JSON",
			ResponseHeader:     "{}",
		}},
	}

	if _, err := app.endpoints.Save(ep, "alice@test.local"); err != nil {
		t.Fatalf("seed endpoint: %v", err)
	}

	return ep
}

func numericResponseParam(t *testing.T, app *application, ep *models.EndPoint) *models.EndPointResponseParam {
	t.Helper()

	stored, err := app.endpoints.Get(ep.ID)
	if err != nil {
		t.Fatalf("reload endpoint: %v", err)
	}

	for _, r := range stored.ResponseMap {
		for _, p := range r.ResponseParams {
			if p.Key == "count" {
				return p
			}
		}
	}

	t.Fatal(`no response parameter for "count"`)
	return nil
}

func postConditionGroup(t *testing.T, ts *httptest.Server, ep *models.EndPoint, param *models.EndPointResponseParam, name, assign string) (int, string) {
	t.Helper()

	form := url.Values{
		"csrf_token":     {csrfToken(t, ts, "/conditiongroups/"+ep.ID+"/add")},
		"name":           {name},
		"httpstatuscode": {ep.ResponseMap[0].ID},
		"responseandconditiongroupmap[0].responsevariable":         {param.ID},
		"responseandconditiongroupmap[0].responsevariablename":     {param.Key},
		"responseandconditiongroupmap[0].responsevariabledatatype": {param.DefaultDatatype},
		"responseandconditiongroupmap[0].assignvalue":              {assign},
	}

	return postForm(t, ts, "/conditiongroups/"+ep.ID+"/add", form)
}

// The response-parameter assign box offers REQUEST[...] and *RANDOM:... values
// via autocomplete, and ApiCall resolves both at request time. Validation must
// therefore not measure them against the field's datatype: "REQUEST[FLOAT64]:
// count" is not itself a number, but the value it resolves to is.
func TestConditionGroupAcceptsRequestAndRandomAssignments(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	createUser(t, app, "alice@test.local", true)
	login(t, ts, "alice@test.local")

	ep := seedEndPointWithNumericResponse(t, app, "NUMERIC")
	param := numericResponseParam(t, app, ep)

	if !strings.EqualFold(param.DefaultDatatype, "FLOAT64") && !strings.EqualFold(param.DefaultDatatype, "INT") {
		t.Fatalf("precondition: count parameter has datatype %q, want a numeric one", param.DefaultDatatype)
	}

	tests := []struct {
		name   string
		assign string
		want   int
	}{
		{"literal number", "42", http.StatusSeeOther},
		{"request variable", "REQUEST[FLOAT64]: count", http.StatusSeeOther},
		{"random value", "*RANDOM:NUMBER", http.StatusSeeOther},
		// Literals are still checked: relaxing validation for dynamic values
		// must not stop a plain wrong value being caught.
		{"non-numeric literal is still rejected", "not a number", http.StatusUnprocessableEntity},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _ := postConditionGroup(t, ts, ep, param, "GROUP"+string(rune('A'+i)), tt.assign)
			if code != tt.want {
				t.Errorf("assigning %q to a %s field: got status %d, want %d",
					tt.assign, param.DefaultDatatype, code, tt.want)
			}
		})
	}
}

// When the form comes back with an error, the parameter rows the user was
// editing must still be there. Re-initialising against an empty response id
// matches no response, which drops every mapping.
func TestConditionGroupKeepsFieldsWhenValidationFails(t *testing.T) {
	app := newTestApplication(t)
	ts := newTestServer(t, app)

	createUser(t, app, "alice@test.local", true)
	login(t, ts, "alice@test.local")

	ep := seedEndPointWithNumericResponse(t, app, "REDISPLAY")
	param := numericResponseParam(t, app, ep)

	// Blank name fails validation, so the page is re-rendered.
	form := url.Values{
		"csrf_token":     {csrfToken(t, ts, "/conditiongroups/"+ep.ID+"/add")},
		"name":           {""},
		"httpstatuscode": {ep.ResponseMap[0].ID},
		"responseandconditiongroupmap[0].responsevariable":         {param.ID},
		"responseandconditiongroupmap[0].responsevariablename":     {param.Key},
		"responseandconditiongroupmap[0].responsevariabledatatype": {param.DefaultDatatype},
		"responseandconditiongroupmap[0].assignvalue":              {"42"},
	}

	code, body := postFormBody(t, ts, "/conditiongroups/"+ep.ID+"/add", form)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d, want %d", code, http.StatusUnprocessableEntity)
	}

	if !strings.Contains(body, param.Key) {
		t.Errorf("the %q parameter row is missing from the re-rendered form: the user's field assignments were discarded", param.Key)
	}
	if !strings.Contains(body, "42") {
		t.Error("the value the user typed was discarded when the form was re-rendered")
	}
}
