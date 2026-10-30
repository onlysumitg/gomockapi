package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/utils/concurrent"
	"github.com/onlysumitg/GoMockAPI/utils/httputils"
	"github.com/onlysumitg/GoMockAPI/utils/jsonutils"
	"github.com/onlysumitg/GoMockAPI/utils/xmlutils"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) APIHandlers(router *chi.Mux) {
	router.Route("/api/{apiname}", func(r chi.Router) {
		//r.With(paginate).Get("/", listArticles)

		r.Get("/", app.GET)
		r.Post("/", app.POST)
		r.Put("/", app.POST)
		r.Patch("/", app.POST)

		r.Delete("/", app.GET)

		r.Get("/*", app.GET)
		r.Post("/*", app.POST)
		r.Put("/*", app.POST)
		r.Patch("/*", app.POST)

		r.Delete("/*", app.GET)
	})

	router.Route("/apilogs", func(r chi.Router) {
		// CSRF
		r.Use(app.RequireAuthentication)

		r.Use(noSurf)
		r.Get("/", app.apilogs)
		r.Get("/{logid}", app.apilogs)
		r.Post("/", app.apilogs)

		logGroup := r.Group(nil)
		logGroup.Use(app.RequireSuperAdmin)
		logGroup.Get("/clear", app.clearapilogs)

	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) apilogs(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	objectid := strings.TrimSpace(r.PostForm.Get("objectid"))
	logid := chi.URLParam(r, "logid")

	if objectid == "" {
		objectid = logid
	}
	logEntries := make([]string, 0)
	if objectid != "" {
		logEntries = models.GetLogs(app.LogDB, objectid)
	}

	data := app.newTemplateData(r)
	data.LogEntries = logEntries

	app.render(w, r, http.StatusOK, "api_logs.tmpl", data)

}

// ------------------------------------------------------
//
// ------------------------------------------------------

func (app *application) InjectClientInfo(r *http.Request, requesyBodyFlatMap map[string]xmlutils.ValueDatatype) {
	requesyBodyFlatMap["*CLIENT_IP"] = xmlutils.ValueDatatype{Value: r.RemoteAddr, DataType: "STRING"}

}

// ------------------------------------------------------
//
// ------------------------------------------------------

func (app *application) GetPathParameters(r *http.Request) (string, string, []httputils.PathParam) {
	namespace := ""
	endpointName := ""
	pathParams := make([]httputils.PathParam, 0)

	params, err := httputils.GetPathParamMap(r.URL.Path, "")
	if err == nil {
		for i, p := range params {
			switch i {
			case 0:
				// do nothing
			case 1:
				namespace = p.Value.(string)
			case 2:
				endpointName = p.Value.(string)

			default:
				p.Name = fmt.Sprintf("*PATH_%d", i-3)
				pathParams = append(pathParams, *p)

			}
		}
	}

	return strings.TrimSpace(namespace), strings.TrimSpace(endpointName), pathParams
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) GET(w http.ResponseWriter, r *http.Request) {

	collection, endpointName, pathParams := app.GetPathParameters(r)
	queryString := fmt.Sprint(r.URL)
	//apiName := chi.URLParam(r, "apiname")

	// apiName, err := httputils.QueryParamPath(queryString, "/api/")
	// if err != nil {
	// 	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
	// 	return
	// }

	requestJson, err := httputils.QueryParamToMap(queryString)
	if err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, err.Error())
		return
	}

	for _, p := range pathParams {
		requestJson[p.Name] = p.Value
	}

	requestBodyFlatMap := jsonutils.JsonToFlatMapFromMap(requestJson)

	app.ProcessAPICall(w, r, collection, endpointName, pathParams, requestBodyFlatMap)

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) POST(w http.ResponseWriter, r *http.Request) {

	collection, endpointName, pathParams := app.GetPathParameters(r)

	endPoint, err := app.GetEndPoint(collection, endpointName, strings.ToLower(r.Method))

	if err != nil {
		app.errorResponse(w, r, 404, err.Error())
		return
	}

	var requestBody any // object or array
	requestBodyFlatMap := make(map[string]xmlutils.ValueDatatype)

	//need to handle xml body

	queryParams, _ := httputils.QueryParamToMap(fmt.Sprint(r.URL))

	isForm, err := parseFormBody(r)
	if err != nil {
		app.errorResponse(w, r, http.StatusBadRequest, "Invalid form body: "+err.Error())
		return
	}

	switch {
	case isForm:
		// form encoded body is accepted regardless of the endpoint's sample request type,
		// field names map to the same flat keys as a JSON object sample request
		requestBodyFlatMap = jsonutils.JsonToFlatMapFromMap(formValuesToMap(r.PostForm))

	case endPoint.SampleRequestType == "JSON":
		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&requestBody)
		switch {
		case err == io.EOF:
			// empty body
		case err != nil:
			app.errorResponse(w, r, http.StatusBadRequest, err.Error())
			return
		}

		requestBodyFlatMap = jsonutils.JsonToFlatMapFromAny(requestBody)

	case endPoint.SampleRequestType == "XML":
		b, err := io.ReadAll(r.Body)
		// b, err := ioutil.ReadAll(resp.Body)  Go.1.15 and earlier
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid XML body [1]")
			return
		}
		requestBodyFlatMap, _, err = xmlutils.XmlToFlatMapAndPlaceholder(string(b))
		if err != nil {
			app.errorResponse(w, r, http.StatusBadRequest, "Invalid XML body [2]")
			return
		}

	}

	// add path parms
	for _, p := range pathParams {
		requestBodyFlatMap[p.Name] = xmlutils.ValueDatatype{Value: p.Value, DataType: "STRING"}
	}

	// add query params
	for k, v := range queryParams {
		requestBodyFlatMap[k] = xmlutils.ValueDatatype{Value: v, DataType: "STRING"}
	}

	app.ProcessAPICall(w, r, collection, endpointName, pathParams, requestBodyFlatMap)

}

// ------------------------------------------------------
// parseFormBody parses the body when the request is
// application/x-www-form-urlencoded or multipart/form-data.
// Returns false (body untouched) for any other content type.
// ------------------------------------------------------
func parseFormBody(r *http.Request) (bool, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false, nil
	}

	switch mediaType {
	case "application/x-www-form-urlencoded":
		return true, r.ParseForm()
	case "multipart/form-data":
		return true, r.ParseMultipartForm(32 << 20)
	}

	return false, nil
}

// ------------------------------------------------------
// formValuesToMap keeps field names as sent. A repeated
// field becomes a list so it flattens to name[0], name[1]...
// ------------------------------------------------------
func formValuesToMap(values map[string][]string) map[string]any {
	asMap := make(map[string]any, len(values))
	for k, v := range values {
		switch len(v) {
		case 0:
			asMap[k] = ""
		case 1:
			asMap[k] = v[0]
		default:
			list := make([]any, len(v))
			for i, item := range v {
				list[i] = item
			}
			asMap[k] = list
		}
	}
	return asMap
}

// ------------------------------------------------------
//
//	actual api call processing
//
// ------------------------------------------------------
func (app *application) ProcessAPICall(w http.ResponseWriter, r *http.Request, collection string, apiName string,
	pathParams []httputils.PathParam,
	requesyBodyFlatMap map[string]xmlutils.ValueDatatype) {

	defer concurrent.Recoverer("Recovered 002")
	defer debug.SetPanicOnFault(debug.SetPanicOnFault(true))

	// ----------------- Header as MAP -------------------Start

	hmap := httputils.GetHeadersAsMap(r)

	for k, v := range hmap {
		requesyBodyFlatMap[fmt.Sprintf("*HEADER_%s", strings.ToUpper(k))] = xmlutils.ValueDatatype{Value: v, DataType: "STRING"}
	}

	// ----------------- Header as MAP -------------------Start

	app.InjectClientInfo(r, requesyBodyFlatMap)
	endPoint, err := app.GetEndPoint(collection, strings.ToLower(apiName), strings.ToLower(r.Method))

	if err != nil {
		app.errorResponse(w, r, 404, err.Error())
		return
	}

	if len(endPoint.ResponseMap) == 0 {
		app.errorResponse(w, r, http.StatusNotImplemented, "No response defined for the endpoint.")
		return
	}

	apiCall := &models.ApiCall{
		ID: uuid.NewString(),
		//Request:         requestJson,
		RequestFlatMap:           requesyBodyFlatMap,
		RequestHeader:            httputils.GetHeadersAsMap(r),
		FinalResponseHeader:      make(map[string]string),
		StatusCode:               http.StatusOK,
		ResponseMessage:          "",
		Log:                      make([]string, 0),
		DB:                       app.LogDB,
		HttpRequest:              r,
		PathParams:               pathParams,
		CurrentEndPoint:          endPoint,
		AdditionalResponseValues: make(map[string]any),
	}

	//apiCall.ResponseString = html.UnescapeString(endPoint.ResponsePlaceholder) //string(jsonByte)
	apiCall.CopyResponseString(endPoint)

	endPoint.BuildResponse(apiCall)

	apiCall.Finalize()

	// JSON or XML ===> TODO
	//app.writeJSON(w, apiCall.ResponseCode, apiCall.Response, apiCall.GetHttpHeader())
	//app.writeJSON(w, apiCall.ResponseCode, apiCall.Response, apiCall.GetHttpHeader())

	app.writeJSONorXML(apiCall.FinalResponseType, w, apiCall.StatusCode, apiCall.FinalResponseString, apiCall.GetHttpHeader())

	go func() {

		defer concurrent.Recoverer("Recovered SaveLogs")
		defer debug.SetPanicOnFault(debug.SetPanicOnFault(true))
		if endPoint.EnableLogging {

			apiCall.SaveLogs()
		}
	}()

	go func() {

		defer concurrent.Recoverer("Recovered 001")

		if endPoint.AutoUpdateRequest {

			//TODO :: for xml
			//app.endpoints.UpdateRequestParamFromApiCall(nil, endPoint, requestJson)
			app.invalidateEndPointCache()
		}
		orignalEndPoint, err := app.endpoints.Get(endPoint.ID)

		if err != nil {
			return
		}

		if !orignalEndPoint.EnableLogging {
			return
		}

		orignalEndPoint.EndPointCallLog = append(orignalEndPoint.EndPointCallLog,
			models.EndPointCallLog{
				CorellationID: apiCall.ID,
				CalledAt:      time.Now().Local(),
			},
		)

		// if apiCall.UsingAcutalUrlResponse && endPoint.AutoUpdateResponse {
		// 	// fetch again to avoid any previous updates
		// 	orignalEndPoint.SampleResponse = apiCall.ActualCallResult.Body
		// 	app.invalidateEndPointCache()
		// }

		app.endpoints.Save(orignalEndPoint, "")

	}()
}

// ------------------------------------------------------
//
// ------------------------------------------------------

func (app *application) clearapilogs(w http.ResponseWriter, r *http.Request) {
	models.ClearLogs(app.DB, app.LogDB)
	app.sessionManager.Put(r.Context(), "flash", "Api logs has been cleared")

	app.goBack(w, r, http.StatusSeeOther)
}
