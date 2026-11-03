package models

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onlysumitg/GoMockAPI/utils/httputils"
	"github.com/onlysumitg/GoMockAPI/utils/xmlutils"
	bolt "go.etcd.io/bbolt"
)

var dealyRangeRegex = regexp.MustCompile(`(?m)\((?P<Start>\d+),(?P<End>\d*)\)`)

type CallResponse struct {
	ID string `json:"id" db:"id" form:"id"` // abc/asd?q=12

	Name string `json:"name" db:"name" form:"Name"` // abc/asd?q=12

	Httpcode int `json:"httpcode" db:"httpcode" form:"httpcode"` // abc/asd?q=12

	ResponseHeader     map[string]string `json:"header" db:"header" form:"header"`
	ResponseHeaderType string            `json:"headertype" db:"headertype" form:"headertype"`

	Response     string `json:"response" db:"response" form:"response"`
	ResponseType string `json:"responsetype" db:"responsetype" form:"responsetype"`
}

type ApiCall struct {
	ID string
	//Request        map[string]any
	RequestFlatMap map[string]xmlutils.ValueDatatype
	RequestHeader  map[string]string

	//Response       map[string]any
	ResponseMapXX []*CallResponse

	FinalResponseString string
	FinalResponseHeader map[string]string
	FinalResponseType   string

	StatusCode int

	ResponseID string

	ResponseMessage string

	//AdditionalDelay int
	AdditionalResponseValues map[string]any

	HasSetCache []string

	Log []string

	PathParams []httputils.PathParam
	logMutex   sync.Mutex

	DB *bolt.DB

	HttpRequest *http.Request

	CurrentEndPoint *EndPoint

	UseActualURL bool

	ActualUrlToUse string

	UsingAcutalUrlResponse bool

	ActualCallResult *httputils.HttpCallResult
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) CopyResponseString(e *EndPoint) {
	a.ResponseMapXX = make([]*CallResponse, len(e.ResponseMap))
	for i, r := range e.ResponseMap {
		c := &CallResponse{
			ID:                 r.ID,
			Httpcode:           r.HttpCode,
			Name:               r.Name,
			ResponseHeader:     make(map[string]string),
			ResponseHeaderType: r.ResponseHeaderType,
			Response:           html.UnescapeString(r.ResponsePlaceholder),
			ResponseType:       r.ResponseType,
		}
		a.ResponseMapXX[i] = c
	}

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) Finalize() {
	a.LogInfo("============== Preparing final response ==============")

	if a.UseActualURL {
		if strings.TrimSpace(a.CurrentEndPoint.ActualURL) != "" {

			a.LogInfo(fmt.Sprintf("Set to call ActualEndPoint %s", a.CurrentEndPoint.ActualURL))

			httpCallResult := a.HTTPCall()

			if httpCallResult != nil {
				if httpCallResult.Err != nil {
					a.LogInfo(fmt.Sprintf("Error calling ActualEndPoint %s. %s", a.CurrentEndPoint.ActualURL, httpCallResult.Err.Error()))
				} else {
					a.LogInfo(fmt.Sprintf("Using response from ActualEndPoint %s", a.ActualUrlToUse))

					a.UsingAcutalUrlResponse = true
					a.ActualCallResult = httpCallResult
					a.StatusCode = httpCallResult.StatusCode

					a.LogInfo(fmt.Sprintf("ResponseCode from ActualEndPoint %d", a.StatusCode))
					a.FinalResponseString = httpCallResult.Body
					//json.Unmarshal([]byte(httpCallResult.Body), &a.Response)
					//a.ResponseHeader = httputils.GetHeadersAsMap2(httpCallResult.Header)

					for k, v := range httputils.GetHeadersAsMap2(httpCallResult.Header) {
						_, found := a.FinalResponseHeader[k]
						if !found {
							a.FinalResponseHeader[k] = v
						}
					}

				}

			}

		} else {
			a.LogInfo("Skipped call ActualEndPoint due to blank URL")

		}

	} else {
		a.ProcessAdditionalResponseValues()
		a.CheckAdditionalDelay()
	}

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) ProcessAdditionalResponseValues() {

	r := a.GetResponseToUse()

	a.LogInfo(fmt.Sprintf("Using http code %d", a.StatusCode))

	a.FinalResponseString = r.Response
	a.FinalResponseType = r.ResponseType
	a.FinalResponseHeader = r.ResponseHeader
	a.StatusCode = r.Httpcode

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) CheckAdditionalDelay() {
	// *DELAY_RESPONSE_MILLI_SEC
	key := fmt.Sprintf("%s_%s", a.ResponseID, "*DELAY_RESPONSE_MILLI_SEC")
	dealyTime, found := a.AdditionalResponseValues[key]
	if found {
		intval, err := strconv.Atoi(dealyTime.(string))
		if err == nil {
			a.LogInfo(fmt.Sprintf("Adding %d millisecond delay", intval))
			time.Sleep(time.Duration(intval) * time.Millisecond)

		} else {
			delayRange, ok := dealyTime.(string)
			if ok {

				start := dealyRangeRegex.SubexpIndex("Start")
				end := dealyRangeRegex.SubexpIndex("End")
				matches := dealyRangeRegex.FindStringSubmatch(delayRange)
				if len(matches) > 0 {

					startTime := matches[start]
					endTime := matches[end]

					startTimeInt, err1 := strconv.Atoi(startTime)
					if err1 != nil || startTimeInt <= 0 {
						startTimeInt = 1
					}
					endTimeInt, err2 := strconv.Atoi(endTime)
					if err2 != nil || endTimeInt <= 0 || endTimeInt <= startTimeInt {
						endTimeInt = startTimeInt + (10 * 1000)
					}

					r := rand.New(rand.NewSource(time.Now().UnixNano()))

					randomDelayInt := r.Intn(endTimeInt-startTimeInt+1) + startTimeInt

					a.LogInfo(fmt.Sprintf("Adding %d millisecond delay[Random]", randomDelayInt))
					time.Sleep(time.Duration(randomDelayInt) * time.Millisecond)

				}
			}

		}
	}
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) HTTPCall() *httputils.HttpCallResult {

	finalUrlToUse, err := a.BuildUrlTOUse()
	if err != nil {
		return nil
	}

	a.ActualUrlToUse = finalUrlToUse

	if a.CurrentEndPoint.isMethod("GET") {
		return a.GETCall(finalUrlToUse)
	}
	if a.CurrentEndPoint.isMethod("POST") {
		return a.POSTCall(finalUrlToUse)
	}
	if a.CurrentEndPoint.isMethod("PUT") {
		return a.PUTCall(finalUrlToUse)
	}
	if a.CurrentEndPoint.isMethod("DELETE") {
		return a.DELETECall(finalUrlToUse)
	}

	return nil
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) BuildUrlTOUse() (string, error) {

	host, found := a.CurrentEndPoint.ParsedUrl["Host"]

	if !found || host == "" {
		a.LogInfo("Skipping as Host var not defined")
		return "", errors.New("host not defined")
	}

	http_https, found := a.CurrentEndPoint.ParsedUrl["Scheme"]
	if !found || http_https == "" {
		http_https = "http"
	}

	// build base URL
	baseUrl := fmt.Sprintf("%s://%s", http_https, host)

	a.LogInfo(fmt.Sprintf("Using base URL %s", baseUrl))

	// get path params

	pathParms := ""

	for i, p := range a.CurrentEndPoint.PathParams {

		if p.IsVariable && len(a.PathParams) >= i+1 {
			pathParms = pathParms + "/" + a.PathParams[i].StringValue
		} else {
			pathParms = pathParms + "/" + p.StringValue
		}

	}

	if pathParms != "" {
		baseUrl = baseUrl + pathParms
	}

	// get query string
	queryString := a.HttpRequest.URL.RawQuery
	if queryString != "" {
		baseUrl = baseUrl + "?" + queryString
	}
	a.LogInfo(fmt.Sprintf("Final URL to use %s", baseUrl))
	return baseUrl, nil
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) GETCall(urlToUse string) *httputils.HttpCallResult {
	var httpCallResult *httputils.HttpCallResult = nil
	if a.CurrentEndPoint.isMethod("GET") {
		httpCallResult = httputils.HttpGET(urlToUse, a.HttpRequest.Header)
	}
	return httpCallResult
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) POSTCall(urlToUse string) *httputils.HttpCallResult {
	var httpCallResult *httputils.HttpCallResult = nil

	if a.CurrentEndPoint.isMethod("POST") {
		body, err := ioutil.ReadAll(a.HttpRequest.Body)
		if err != nil {
			body = []byte("")
		}

		httpCallResult = httputils.HttpPOST(urlToUse, a.HttpRequest.Header, body)
	}
	return httpCallResult
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) PUTCall(urlToUse string) *httputils.HttpCallResult {
	var httpCallResult *httputils.HttpCallResult = nil

	if a.CurrentEndPoint.isMethod("PUT") {
		body, err := ioutil.ReadAll(a.HttpRequest.Body)
		if err != nil {
			body = []byte("")
		}

		httpCallResult = httputils.HttpPUT(urlToUse, a.HttpRequest.Header, body)
	}
	return httpCallResult
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (a *ApiCall) DELETECall(urlToUse string) *httputils.HttpCallResult {
	var httpCallResult *httputils.HttpCallResult = nil

	if a.CurrentEndPoint.isMethod("DELETE") {
		body, err := ioutil.ReadAll(a.HttpRequest.Body)
		if err != nil {
			body = []byte("")
		}

		httpCallResult = httputils.HttpDELETE(urlToUse, a.HttpRequest.Header, body)
	}
	return httpCallResult
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (apiCall *ApiCall) GetResponseToUse() *CallResponse {
	if apiCall.StatusCode <= 0 {
		apiCall.StatusCode = 200
	}
	for _, r := range apiCall.ResponseMapXX {
		if r.ID == apiCall.ResponseID {
			apiCall.StatusCode = r.Httpcode
			return r
		}
	}
	dResponse := apiCall.CurrentEndPoint.GetDefaultResponseID()
	for _, r := range apiCall.ResponseMapXX {
		if r.ID == dResponse.ID {
			apiCall.ResponseID = r.ID
			apiCall.StatusCode = r.Httpcode
			return r
		}
	}

	r := apiCall.ResponseMapXX[0]
	apiCall.StatusCode = r.Httpcode
	apiCall.ResponseID = r.ID
	return r

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (apiCall *ApiCall) GetHttpHeader() http.Header {
	var header http.Header = make(http.Header)

	header["CORRELATIONID"] = []string{apiCall.ID}

	for key, value := range apiCall.FinalResponseHeader {
		header[key] = []string{value}
	}

	delete(header, "Content-Length")

	return header
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (apiCall *ApiCall) HasSet(keyToCheck string) bool {
	hasSet := false
	for _, key := range apiCall.HasSetCache {
		if strings.EqualFold(key, keyToCheck) {
			hasSet = true
			break
		}

	}

	return hasSet
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (apiCall *ApiCall) SetKey(keyToCheck string) {
	apiCall.HasSetCache = append(apiCall.HasSetCache, keyToCheck)
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func getLogTableName() []byte {
	return []byte("apilogs")
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (apiCall *ApiCall) LogInfo(logEntry string) {

	defer apiCall.logMutex.Unlock()
	apiCall.logMutex.Lock()

	entry := fmt.Sprintf("INFO \t%s %s\n", time.Now().Format("2006/01/02 15:04:05"), logEntry)
	apiCall.Log = append(apiCall.Log, entry)

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (apiCall *ApiCall) LogError(logEntry string) {
	defer apiCall.logMutex.Unlock()
	apiCall.logMutex.Lock()

	entry := fmt.Sprintf("ERROR\t%s %s\n", time.Now().Format("2006/01/02 15:04:05"), logEntry)
	apiCall.Log = append(apiCall.Log, entry)

}

// ------------------------------------------------------
//
// ------------------------------------------------------
// func (l *ErrorLogger) Error(err error) {
// 	// Get the stack trace as a string
// 	//buf := new(bytes.Buffer)
// 	//l.logger.withStack(buf, err)

// 	//sendErrorMail(buf.String())

//		buf := bytes.NewBufferString(s)
//	}
//
// ------------------------------------------------------
//
// ------------------------------------------------------
func (m *ApiCall) SaveLogs() {
	m.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(getLogTableName())
		if err != nil {
			return err
		}

		for i, s := range m.Log {

			key := fmt.Sprintf("%s_%d", m.ID, i)
			bucket.Put([]byte(key), []byte(fmt.Sprintf("%05d. %s", i+1, s)))

		}
		return nil
	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func GetLogs(db *bolt.DB, id string) []string {

	l := make([]string, 0)

	_ = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(getLogTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, []byte(id)) {
				l = append(l, string(v))
			}
		}

		return nil
	})

	return l

}
