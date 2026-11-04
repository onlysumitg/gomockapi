package models

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onlysumitg/GoMockAPI/internal/validator"
	"github.com/onlysumitg/GoMockAPI/utils/httputils"
	"github.com/onlysumitg/GoMockAPI/utils/stringutils"
)

type EndPointCallLog struct {
	CorellationID string
	CalledAt      time.Time
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// Define a new User type. Notice how the field names and types align
// with the columns in the database "users" table?
type EndPoint struct {
	ID             string `json:"id" db:"id" form:"id"`
	CollectionID   string `json:"collectionid" db:"collectionid" form:"collectionid"`
	CollectionName string `json:"collectionname" db:"collectionname" form:"-"`

	Name string `json:"name" db:"name" form:"name"` // abc/asd?q=12
	//Path string `json:"path" db:"path" form:"path"` // abc

	Method string `json:"method" db:"method" form:"method"`
	OnHold bool   `json:"onhold" db:"onhold" form:"onhold"`

	ActualURL  string                `json:"actualurl" db:"actualurl" form:"actualurl"`
	ParsedUrl  map[string]string     `json:"parsedurl" db:"parsedurl" form:"-"`
	PathParams []httputils.PathParam `json:"pathparams" db:"pathparams" form:"-"`

	MockUrl string `json:"mockurl" db:"mockurl" form:"-"`

	AutoUpdateRequest  bool `json:"autoupdaterequest" db:"autoupdaterequest" form:"autoupdaterequest"`
	AutoUpdateResponse bool `json:"autoupdateresponse" db:"autoupdateresponse" form:"autoupdateresponse"`

	SampleRequest string `json:"samplerequest" db:"samplerequest" form:"samplerequest"`
	//RequestPlaceholder map[string]any `json:"requestplaceholder" db:"requestplaceholder" form:"requestplaceholder"`
	SampleRequestType string `json:"samplerequesttype" db:"samplerequesttype" form:"samplerequesttype"`

	SampleRequestHeader      string         `json:"samplerequestheader" db:"samplerequestheader" form:"samplerequestheader"`
	RequestHeaderPlaceholder map[string]any `json:"requestheaderplaceholder" db:"requestheaderplaceholder" form:"requestheaderplaceholder"`
	SampleRequestHeaderType  string         `json:"samplerequestheadertype" db:"samplerequestheadertype" form:"samplerequestheadertype"`

	// SampleResponse      string `json:"sampleresponse" db:"sampleresponse" form:"sampleresponse"`
	// ResponsePlaceholder string `json:"responseplaceholder" db:"responseplaceholder" form:"responseplaceholder"`
	// SampleResponseType  string `json:"sampleresponsetype" db:"sampleresponsetype" form:"sampleresponsetype"`

	// SampleResponseHeader      string `json:"sampleresponseheader" db:"sampleresponseheader" form:"sampleresponseheader"`
	// ResponseHeaderPlaceholder string `json:"responseheaderplaceholder" db:"responseheaderplaceholder" form:"responseheaderplaceholder"`
	// SampleResponseHeaderType  string `json:"sampleresponseheadertype" db:"sampleresponseheadertype" form:"sampleresponseheadertype"`

	AppendReqParam bool `json:"appendrequest" db:"appendrequest" form:"appendrequest"`
	AppendResParam bool `json:"appendresponse" db:"appendresponse" form:"appendresponse"`

	AppendReqHeader bool `json:"appendrequestheader" db:"appendrequestheader" form:"appendrequestheader"`
	AppendResHeader bool `json:"appendresponseheader" db:"appendresponseheader" form:"appendresponseheader"`

	validator.Validator

	RequestParams []*EndPointRequestParam `json:"-" db:"-" from:"-"`
	//ResponseParams []*EndPointResponseParam `json:"-" db:"-" from:"-"`

	ResponseMap []*EndPointResponse `json:"sampleresponse" db:"sampleresponse" form:"sampleresponse"`

	ConditionGroups []*ConditionGroup `json:"-" db:"-" from:"-"`
	EndPointCallLog []EndPointCallLog `json:"endpointcalllog" db:"endpointcalllog" form:"-"`

	CreatedBy string    `json:"createdby" db:"createdby" form:"-"`
	CreatedOn time.Time `json:"createdon" db:"createdon" form:"-"`

	EnableLogging bool `json:"enablelogging" db:"enablelogging" form:"enablelogging"`
}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s EndPoint) ClearCache() {
	//database.ClearCache(s)
}

// IsCacheable returns true when the endpoint's response can be cached
// based on condition-relevant request values alone. Endpoints that use
// RANDOM values, REQUEST[] references, or proxy (CallActualEndPoint)
// are not cacheable.
func (s *EndPoint) IsCacheable() bool {
	for _, cg := range s.ConditionGroups {
		if cg.CallActualEndPoint {
			return false
		}
		for _, p := range cg.ConditionGroupParameters {
			if IsDynamicAssignment(p.AssgineValue) {
				return false
			}
		}
	}
	for _, r := range s.ResponseMap {
		for _, rp := range r.ResponseParams {
			if IsDynamicAssignment(rp.OverrideValue) {
				return false
			}
		}
	}
	return true
}

// ConditionKeys returns the deduplicated set of request flat-map keys
// that this endpoint's conditions inspect. These keys form the cache key.
func (s *EndPoint) ConditionKeys() []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, cg := range s.ConditionGroups {
		for _, c := range cg.Conditions {
			if c.RequestParam != nil {
				if _, ok := seen[c.RequestParam.Key]; !ok {
					seen[c.RequestParam.Key] = struct{}{}
					keys = append(keys, c.RequestParam.Key)
				}
			}
		}
	}
	return keys
}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) GetResponseByID(id string) *EndPointResponse {
	for _, r := range s.ResponseMap {
		if strings.EqualFold(r.ID, id) {
			return r
		}
	}
	return &EndPointResponse{HttpCode: 200}
}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) IsDuplicateHTTPCodeResponse(httpCode int, current *EndPointResponse) bool {
	isDuplicate := false
	for _, r := range s.ResponseMap {
		if r.ID != current.ID && r.HttpCode == httpCode && strings.EqualFold(r.Name, current.Name) {
			isDuplicate = true
			break
		}
	}
	return isDuplicate

}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) IsDuplicateResponseWithNameDefault(httpCode int, current *EndPointResponse) bool {

	if !strings.EqualFold(current.Name, "DEFAULT") {
		return false
	}

	isDuplicate := false
	for _, r := range s.ResponseMap {
		if r.ID != current.ID && strings.EqualFold(r.Name, current.Name) {
			isDuplicate = true
			break
		}
	}
	return isDuplicate

}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) RemoveResponse(id string) {

	newlist := make([]*EndPointResponse, 0)

	for _, r := range s.ResponseMap {
		if !strings.EqualFold(r.ID, id) {
			newlist = append(newlist, r)
		}
	}

	s.ResponseMap = newlist

}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) SetResponse(response *EndPointResponse) {
	if response.ID == "" {
		response.ID = strings.ToUpper(uuid.NewString())
	}

	newlist := make([]*EndPointResponse, 0)

	for _, r := range s.ResponseMap {
		if !strings.EqualFold(r.ID, response.ID) {
			newlist = append(newlist, r)
		}
	}

	newlist = append(newlist, response)

	s.ResponseMap = newlist

}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) BuildMockUrl() {

	pathParamString, foundPathParam := s.ParsedUrl["Path"]
	if !foundPathParam {

		pathParamString = ""
	}

	if pathParamString != "" && !strings.HasPrefix(pathParamString, "/") {
		pathParamString = "/" + pathParamString
	}

	queryParamString, foundQueryParam := s.ParsedUrl["RawQuery"]
	if foundQueryParam && queryParamString != "" {
		queryParamString = "?" + queryParamString
	} else {
		queryParamString = ""
	}
	if s.CollectionName == "" {
		s.CollectionName = "V1"
	}

	s.MockUrl = fmt.Sprintf("api/%s/%s%s%s", s.CollectionName, s.Name, pathParamString, queryParamString)
}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s EndPoint) isMethod(m string) bool {
	return strings.EqualFold(strings.TrimSpace(s.Method), strings.TrimSpace(m))
}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s EndPoint) BuildResponse(apiCall *ApiCall) (err error) {

	apiCall.LogInfo(fmt.Sprintf("Starting endpoint: %s", s.Name))

	// process every condition group
	// short condition group list by name

	apiCall.LogInfo("**** STARTED checking condition groups ****")

	for _, cg := range s.ConditionGroups {
		apiCall.LogInfo(fmt.Sprintf("======> Started Processing Condition Group: %s", cg.Name))

		_ = cg.Execute(apiCall)
		apiCall.LogInfo(fmt.Sprintf("======> Finished Processing Condition Group: %s  ", cg.Name))
		apiCall.LogInfo("-----")

	}

	apiCall.LogInfo("**** FINISHED checking condition groups ****")

	apiCall.LogInfo("===================================================")

	// process each response paramater
	// if placeholder still have the response param
	apiCall.LogInfo("**** STARTED checking default values ****")

	for _, r := range s.ResponseMap {
		apiCall.LogInfo(fmt.Sprintf("======> Processing Response for code %s %d  Started", r.Name, r.HttpCode))

		for _, rp := range r.ResponseParams {
			apiCall.LogInfo(fmt.Sprintf("==> Processing Response Variable for Default: %s  Started", rp.Key))
			rp.process(apiCall, r.ID, "")
			apiCall.LogInfo(fmt.Sprintf("==> Processing Response Variable for Default: %s  Finished", rp.Key))
			apiCall.LogInfo("-----")

		}
		apiCall.LogInfo(fmt.Sprintf("======> Processing Response for code %s %d  Finished", r.Name, r.HttpCode))

	}

	apiCall.LogInfo("**** FINISHED checking default values ****")

	// build reponse map
	// err = json.Unmarshal([]byte(apiCall.ResponseString), &apiCall.Response)

	// if err != nil {
	// 	//apiCall.Response["err2"] = err.Error()

	// }

	return err

	//responseJson = s.ResponsePlaceholder
	//return responseJson

}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) PreValidations() {

	endpoint.SampleRequestType = strings.ToUpper(endpoint.SampleRequestType)
	endpoint.SampleRequestHeaderType = strings.ToUpper(endpoint.SampleRequestHeaderType)

	// endpoint.SampleResponseType = strings.ToUpper(endpoint.SampleResponseType)
	// endpoint.SampleResponseHeaderType = strings.ToUpper(endpoint.SampleResponseHeaderType)

	endpoint.Method = strings.ToUpper(endpoint.Method)

	// Get request type is always JSON
	if endpoint.Method == "GET" {
		endpoint.SampleRequestType = "JSON"
	}

	endpoint.CheckField(validator.NotBlank(endpoint.Name), "name", "This field cannot be blank")
	endpoint.CheckField(validator.MustNotStartwith(endpoint.Name, "/"), "name", "Can not start with / or {")
	endpoint.CheckField(validator.MustNotStartwith(endpoint.Name, "{"), "name", "Can not start with / or {")

	endpoint.CheckField(validator.NotBlank(endpoint.ActualURL), "actualurl", "This field cannot be blank")
	endpoint.CheckField(validator.MustStartwithOneOf(endpoint.ActualURL, "HTTP://", "HTTPS://"), "actualurl", "Must start with http:// or https://")

	endpoint.CheckField(validator.MustNotStartwith(endpoint.ActualURL, "/"), "actualurl", "Can not start with / or {")
	endpoint.CheckField(validator.MustNotStartwith(endpoint.ActualURL, "{"), "actualurl", "Can not start with / or {")

	endpoint.CheckField(validator.NotBlank(endpoint.Method), "method", "This field cannot be blank")
	endpoint.CheckField(validator.MustBeFromList(endpoint.Method, "POST", "GET", "PUT", "DELETE"), "method", "Valid values are POST, GET, PUT, DELETE")

	endpoint.CheckField(validator.NotBlank(endpoint.SampleRequestType), "samplerequesttype", "Please select one")
	endpoint.CheckField(validator.MustBeFromList(endpoint.SampleRequestType, "JSON", "XML"), "samplerequesttype", "Valid values are JSON or XML")

	endpoint.CheckField(validator.NotBlank(endpoint.SampleRequestHeaderType), "samplerequestheadertype", "Please select one")
	endpoint.CheckField(validator.MustBeFromList(endpoint.SampleRequestHeaderType, "JSON", "XML"), "samplerequestheadertype", "Valid values are JSON or XML")

	// endpoint.CheckField(validator.NotBlank(endpoint.SampleResponseType), "sampleresponsetype", "Please select one")
	// endpoint.CheckField(validator.MustBeFromList(endpoint.SampleResponseType, "JSON", "XML"), "sampleresponsetype", "Valid values are JSON or XML")

	// endpoint.CheckField(validator.NotBlank(endpoint.SampleResponseHeaderType), "sampleresponseheadertype", "Please select one")
	// endpoint.CheckField(validator.MustBeFromList(endpoint.SampleResponseHeaderType, "JSON", "XML"), "sampleresponseheadertype", "Valid values are JSON or XML")

}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) PostValidations() {

	if endpoint.SampleRequestType == "JSON" {
		endpoint.CheckField(validator.MustBeJSON(endpoint.SampleRequest), "samplerequest", "Must be a valid JSON")
	}

	if endpoint.SampleRequestHeaderType == "JSON" {
		endpoint.CheckField(validator.MustBeJSON(endpoint.SampleRequestHeader), "samplerequestheader", "Must be a valid JSON")
	}

	if endpoint.SampleRequestType == "XML" {
		endpoint.CheckField(validator.MustBeXML(endpoint.SampleRequest), "samplerequest", "Must be a valid XML")
	}

	if endpoint.SampleRequestHeaderType == "XML" {
		endpoint.CheckField(validator.MustBeXML(endpoint.SampleRequestHeader), "samplerequestheader", "Must be a valid XML")
	}

	// if endpoint.SampleResponseType == "JSON" {
	// 	endpoint.CheckField(validator.MustBeJSON(endpoint.SampleResponse), "sampleresponse", "Must be a valid JSON")
	// }
	// if endpoint.SampleResponseHeaderType == "JSON" {
	// 	endpoint.CheckField(validator.MustBeJSON(endpoint.SampleResponseHeader), "sampleresponseheader", "Must be a valid JSON")
	// }
	// if endpoint.SampleResponseType == "XML" {
	// 	endpoint.CheckField(validator.MustBeXML(endpoint.SampleResponse), "sampleresponse", "Must be a valid XML")
	// }
	// if endpoint.SampleResponseHeaderType == "XML" {
	// 	endpoint.CheckField(validator.MustBeXML(endpoint.SampleResponseHeader), "sampleresponseheader", "Must be a valid XML")
	// }
}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) ProcessPathParams() {

	p, err := httputils.GetPathParamMap(endpoint.ActualURL, "")

	endpoint.PathParams = make([]httputils.PathParam, 0)

	if err == nil {
		for _, x := range p {
			endpoint.PathParams = append(endpoint.PathParams, *x)

		}
	} else {
		endpoint.CheckField(false, "actualurl", err.Error())
	}

}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) Prepare() {
	endpoint.Method = strings.ToUpper(endpoint.Method)
	endpoint.PreValidations()

	err := endpoint.ParseUrl()
	if err != nil {
		endpoint.CheckField(false, "actualurl", err.Error())
	}

	if !endpoint.Valid() {
		return
	}

	endpoint.ProcessPathParams()

	switch endpoint.Method {
	case "GET":
		endpoint.preapreGETEndpoint()

	case "POST":
		endpoint.preaprePOSTEndpoint()

	case "PATCH":
		endpoint.preaprePOSTEndpoint()

	case "PUT":
		endpoint.preaprePUTEndpoint()

	case "DELETE":
		endpoint.preapreGETEndpoint()
	}

	endpoint.Name = strings.Trim(endpoint.Name, "/")
	endpoint.Name = stringutils.RemoveSpecialChars(stringutils.RemoveMultipleSpaces(endpoint.Name))
	endpoint.PostValidations()
}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) preapreGETEndpoint() {
	requestJson, errQ := httputils.QueryParamToJson(endpoint.ActualURL)

	if errQ != nil {
		endpoint.CheckField(false, "name", errQ.Error())

	} else {
		endpoint.SampleRequest = requestJson
	}

	// pathName, errP := httputils.QueryParamPath(endpoint.Name, "")
	// if errP != nil {
	// 	endpoint.CheckField(false, "name", errP.Error())

	// } else {
	// 	endpoint.Path = pathName
	// }
}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) preaprePOSTEndpoint() {
	//endpoint.Path = endpoint.Name
}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) preaprePUTEndpoint() {
	//endpoint.Path = endpoint.Name
}

// ----------------------------------------------
//
// ----------------------------------------------
func (endpoint *EndPoint) preapreDELETEEndpoint() {
	//endpoint.Path = endpoint.Name
}

// ----------------------------------------------
//
// ----------------------------------------------
func (e *EndPoint) ParseUrl() error {

	parsedURL, err := url.Parse(e.ActualURL)

	if err != nil {
		return err
	}

	e.ParsedUrl = make(map[string]string)

	e.ParsedUrl["Scheme"] = parsedURL.Scheme
	e.ParsedUrl["Opaque"] = parsedURL.Opaque

	//e.ParsedUrl["User"] = parsedURL.User

	e.ParsedUrl["Host"] = parsedURL.Host
	e.ParsedUrl["Path"] = parsedURL.Path
	e.ParsedUrl["RawPath"] = parsedURL.RawPath
	//e.ParsedUrl["ForceQuery"] = parsedURL.ForceQuery
	e.ParsedUrl["RawQuery"] = parsedURL.RawQuery
	e.ParsedUrl["Fragment"] = parsedURL.Fragment
	e.ParsedUrl["RawFragment"] = parsedURL.RawFragment

	return nil
}

// ------------------------------------------------------------
// Build Response Placeholder
// ------------------------------------------------------------
func (e *EndPoint) BuildResponsePlaceholder() {
	for _, v := range e.ResponseMap {
		v.BuildResponsePlaceholder()
		v.BuildResponseHeaderPlaceholder()
	}
}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPoint) GetDefaultResponseID() *EndPointResponse {

	// with the name default
	for _, r := range s.ResponseMap {
		if strings.EqualFold(r.Name, "DEFAULT") {
			return r
		}
	}

	// first with HTTP code 200
	for _, r := range s.ResponseMap {
		if r.HttpCode == 200 {
			return r
		}
	}

	//	first with HTTP code 2XX

	for _, r := range s.ResponseMap {
		if r.HttpCode >= 200 && r.HttpCode <= 299 {
			return r
		}
	}

	// 1st one in the list

	if len(s.ResponseMap) > 0 {
		return s.ResponseMap[0]
	}

	return &EndPointResponse{}
}
