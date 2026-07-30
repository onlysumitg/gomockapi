package httputils

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"
)

type HttpCallResult struct {
	Header     http.Header
	StatusCode int
	Body       string
	Err        error
}

//--------------------------------------------------------
//
//--------------------------------------------------------

func HttpGET(url string, header http.Header) *HttpCallResult {

	httpCallResult := &HttpCallResult{}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		httpCallResult.Err = err
		return httpCallResult
	}

	req.Header = header

	return HttpProcessRequest(req)

}

// --------------------------------------------------------
//
// --------------------------------------------------------
func HttpPOST(url string, header http.Header, requestPayLoad []byte) *HttpCallResult {
	httpCallResult := &HttpCallResult{}
	bodyReader := bytes.NewReader(requestPayLoad)

	req, err := http.NewRequest(http.MethodPost, url, bodyReader)
	if err != nil {
		httpCallResult.Err = err
		return httpCallResult
	}

	req.Header = header

	return HttpProcessRequest(req)
}

// --------------------------------------------------------
//
// --------------------------------------------------------
func HttpPUT(url string, header http.Header, requestPayLoad []byte) *HttpCallResult {
	httpCallResult := &HttpCallResult{}
	bodyReader := bytes.NewReader(requestPayLoad)

	req, err := http.NewRequest(http.MethodPut, url, bodyReader)
	if err != nil {
		httpCallResult.Err = err
		return httpCallResult
	}

	req.Header = header

	return HttpProcessRequest(req)
}

// --------------------------------------------------------
//
// --------------------------------------------------------
func HttpDELETE(url string, header http.Header, requestPayLoad []byte) *HttpCallResult {
	httpCallResult := &HttpCallResult{}
	bodyReader := bytes.NewReader(requestPayLoad)

	req, err := http.NewRequest(http.MethodDelete, url, bodyReader)
	if err != nil {
		httpCallResult.Err = err
		return httpCallResult
	}

	req.Header = header

	return HttpProcessRequest(req)
}

//--------------------------------------------------------
//
//--------------------------------------------------------

func HttpProcessRequest(req *http.Request) *HttpCallResult {
	httpCallResult := &HttpCallResult{}

	if req.Header == nil {
		req.Header = make(http.Header)
	}

	// Only supply a Content-Type when the caller forwarded none. Overwriting it
	// unconditionally mislabelled XML endpoints as JSON upstream.
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		httpCallResult.Err = err
		return httpCallResult
	}

	httpCallResult.StatusCode = res.StatusCode
	httpCallResult.Header = res.Header

	body, err := ioutil.ReadAll(res.Body)
	if err == nil {
		httpCallResult.Body = string(body)
	}

	res.Body.Close()

	return httpCallResult
}
