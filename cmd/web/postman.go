package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/utils/concurrent"
	"github.com/onlysumitg/GoMockAPI/utils/httputils"
	"github.com/onlysumitg/GoMockAPI/utils/stringutils"
	postman "github.com/rbretecher/go-postman-collection"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) PostmantHandlers(router *chi.Mux) {
	router.Route("/postman", func(r chi.Router) {
		//r.With(paginate).Get("/", listArticles)
		r.Use(app.sessionManager.LoadAndSave)

		r.Use(app.RequireAuthentication)
 
		r.Get("/", app.fileUploader)

		r.Get("/test", app.PaypalTest)

		r.Get("/d", app.downloadPostmanCollection)

		r.Post("/upload", app.uploadHandler)
		r.Post("/webget", app.getFromWeb)

	})

}

// ------------------------------------------------------
// download file
// ------------------------------------------------------
func (app *application) downloadPostmanCollection(w http.ResponseWriter, r *http.Request) {

	c := app.EndpointsToPostmanCollection()

	fileName := "GoMockAPI.json"

	buf := bytes.NewBuffer(nil)

	err := c.Write(buf, postman.V210)

	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error %s", err.Error()))
		app.goBack(w, r, http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Description", "File Transfer")                  // can be used multiple times
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName) // can be used multiple times
	w.Header().Set("Content-Type", "application/octet-stream")

	w.Write(buf.Bytes())
}

// https://learning.postman.com/collection-format/getting-started/structure-of-a-collection/

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) EndpointsToPostmanCollection() *postman.Collection {
	c := postman.CreateCollection(fmt.Sprintf("GoMockAPI"), "GOMockAPI Postman collection")
	c.Variables = make([]*postman.Variable, 0)

	// Snapshot rather than ranging over the live cache, and keep the fallback
	// name local: these endpoints are shared with in-flight requests.
	for _, ep := range app.cachedEndPoints() {
		collectionName := ep.CollectionName
		if collectionName == "" {
			collectionName = "V1"
		}
		folder := c.AddItemGroup(collectionName)

		folder.AddItem(app.EndPointToItem(ep))

	}

	return c
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) fileUploader(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)

	app.render(w, r, http.StatusOK, "postman_upload_json.tmpl", data)
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------
func (app *application) EndPointToItem(ep *models.EndPoint) *postman.Items {

	/*
			Name                    string      `json:"name"`
		Description             string      `json:"description,omitempty"`
		Variables               []*Variable `json:"variable,omitempty"`
		Events                  []*Event    `json:"event,omitempty"`
		ProtocolProfileBehavior interface{} `json:"protocolProfileBehavior,omitempty"`
		ID                      string      `json:"id,omitempty"`
		Request                 *Request    `json:"request,omitempty"`
		Responses               []*Response `json:"response,omitempty"`
	*/

	urlAddress := ep.MockUrl

	urlAddress = fmt.Sprintf("%s/%s", app.hostURL, urlAddress)

	host := ""
	port := ""
	path := make([]string, 0)

	query := make([]*postman.QueryParam, 0)

	queryPrams, err := httputils.QueryParamToMap(urlAddress)
	if err == nil {
		for k, v := range queryPrams {
			q := &postman.QueryParam{
				Key:   k,
				Value: (v).(string),
			}
			query = append(query, q)

		}
	}

	u, err := url.Parse(urlAddress)
	if err == nil {

		// fmt.Println(">>>>>>>Scheme>>>>>>>", u.Scheme)

		// fmt.Println(">>>>>>>Opaque>>>>>>>", u.Opaque)

		if strings.Contains(u.Host, ":") {
			broken := strings.Split(u.Host, ":")
			host = broken[0]
			port = broken[1]

		}
		// fmt.Println(">>>>>>>>Host>>>>>>", u.Host)
		// fmt.Println(">>>>>>>>>>Path>>>>", u.Path)

		path = strings.Split(u.Path, "/")

		// fmt.Println(">>>>>>>>RawPath>>>>>>", u.RawPath)
		// fmt.Println(">>>>>>>>OmitHost>>>>>>", u.OmitHost)
		// fmt.Println(">>>>>>>>>ForceQuery>>>>>", u.ForceQuery)
		// fmt.Println(">>>>>>RawQuery>>>>>>>>", u.RawQuery)
		// fmt.Println(">>>>>>>>Fragment>>>>>>", u.Fragment)
		// fmt.Println(">>>>>>>>>>>RawFragment>>>", u.RawFragment)

	}

	postManItem := postman.CreateItem(postman.Item{
		Name:        ep.Name,
		Description: ep.Name,
		ID:          ep.ID,
		Request: &postman.Request{
			URL: &postman.URL{
				Raw:      urlAddress,
				Protocol: u.Scheme,
				Host:     []string{host},
				Port:     port,
				Path:     path,
				Query:    query,
			},
			Method: postman.Method(strings.ToUpper(ep.Method)),
			Auth:   postman.CreateAuth(postman.Bearer, postman.CreateAuthParam("bearer", "{{authtoken}}")),
			Body: &postman.Body{
				Mode:    "raw",
				Raw:     ep.SampleRequest,
				Options: &postman.BodyOptions{Raw: postman.BodyOptionsRaw{Language: ep.SampleRequestType}},
			},
		},
	})

	return postManItem
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

const MAX_UPLOAD_SIZE = 1024 * 1024 * 5 // 5MB

// Progress is used to track the progress of a file upload.
// It implements the io.Writer interface so it can be passed
// to an io.TeeReader()
type Progress struct {
	TotalSize int64
	BytesRead int64
}

// Write is used to satisfy the io.Writer interface.
// Instead of writing somewhere, it simply aggregates
// the total bytes on each read
func (pr *Progress) Write(p []byte) (n int, err error) {
	n, err = len(p), nil
	pr.BytesRead += int64(n)
	pr.Print()
	return
}

// Print displays the current progress of the file upload
func (pr *Progress) Print() {
	if pr.BytesRead == pr.TotalSize {
		fmt.Println("DONE!")
		return
	}

	fmt.Printf("File upload in progress: %d\n", pr.BytesRead)
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) uploadHandler(w http.ResponseWriter, r *http.Request) {
	user, err := app.GetUser(r)
	if err != nil {
		app.UnauthorizedError(w, r)
		return
	}
	// 32 MB is the default used by FormFile
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusSeeOther)
		return
	}

	// get a reference to the fileHeaders
	files := r.MultipartForm.File["file"]

	for _, fileHeader := range files {
		if fileHeader.Size > MAX_UPLOAD_SIZE {

			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("The uploaded file is too big: %s. Please use an file less than 5MB in size", fileHeader.Filename))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error processing file %s", err.Error()))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		defer file.Close()

		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error reading file %s", err.Error()))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}
		//json ==> "text/plain; charset=utf-8"

		fmt.Println("filepath.Ext(fileHeader.Filename) ", filepath.Ext(fileHeader.Filename))
		if filepath.Ext(fileHeader.Filename) != ".json" {
			app.sessionManager.Put(r.Context(), "error", "Only json files are allowed.")
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		// filetype := http.DetectContentType(buff)
		// if filetype != "image/jpeg" && filetype != "image/png" {
		// 	http.Error(w, "The provided file format is not allowed. Please upload a JPEG or PNG image", http.StatusBadRequest)
		// 	return
		// }

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error reading file %s", err.Error()))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		err = os.MkdirAll("./uploads", os.ModePerm)
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error uploading file %s", err.Error()))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		fileName := fmt.Sprintf("./uploads/%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename))
		f, err := os.Create(fileName)
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error uploading file %s", err.Error()))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		defer f.Close()

		pr := &Progress{
			TotalSize: fileHeader.Size,
		}

		_, err = io.Copy(f, io.TeeReader(file, pr))
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error saving file %s", err.Error()))
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

		messages := app.ReadPostmantJson(f.Name(), user)

		data := app.newTemplateData(r)
		data.Messages = messages
		app.render(w, r, http.StatusOK, "user_message.tmpl", data)

	}

	app.goBack(w, r, http.StatusSeeOther)

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) PaypalTest(w http.ResponseWriter, r *http.Request) {
	user, err := app.GetUser(r)
	if err != nil {
		app.UnauthorizedError(w, r)
		return
	}
	app.ReadPostmantJson("./uploads/paypal.json", user)

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) getFromWeb(w http.ResponseWriter, r *http.Request) {

	user, err := app.GetUser(r)
	if err != nil {
		app.UnauthorizedError(w, r)
		return
	}

	err = r.ParseForm()

	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusSeeOther)
		return
	}

	url := r.PostForm.Get("url")
	if url == "" {
		app.sessionManager.Put(r.Context(), "error", "Invalid url")
		app.goBack(w, r, http.StatusSeeOther)
		return
	}

	filePath := fmt.Sprintf("./uploads/%d%s", time.Now().UnixNano(), ".json")

	err = httputils.DownloadFile(filePath, url)
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", err.Error())
		app.goBack(w, r, http.StatusSeeOther)
		return
	}

	messages := app.ReadPostmantJson(filePath, user)
	app.sessionManager.Put(r.Context(), "flash", "Done")
	data := app.newTemplateData(r)

	data.Messages = messages
	app.render(w, r, http.StatusOK, "user_message.tmpl", data)

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) ReadPostmantJson(filename string, currentUser *models.User) []string {

	messageList := make([]string, 0)
	// https://learning.postman.com/collection-format/getting-started/overview/

	fmt.Println(">>>filename?>>", filename)
	file, err := os.Open(filename)
	defer concurrent.Recoverer("ReadPostmantJson")

	defer func() {
		file.Close()
		os.Remove(filename)

	}()

	if err != nil {
		fmt.Println("error1", err, filename)
		messageList = append(messageList, fmt.Sprintf("Error: %s", err.Error()))
		return messageList
	}

	c, err := postman.ParseCollection(file)
	if err != nil {
		messageList = append(messageList, fmt.Sprintf("Error: %s", err.Error()))
		return messageList
	}

	variabels := make(map[string]string)
	for _, v := range c.Variables {
		k := v.Key
		if k == "" {
			k = v.Name
		}
		if k == "" {
			continue
		}

		k = fmt.Sprintf("{{%s}}", k)
		variabels[k] = v.Value
	}

	// fmt.Println("Postman connection info ==== start")
	// fmt.Println("c.Info.Name", c.Info.Name)
	// fmt.Println("c.Info.Schema", c.Info.Schema)
	// fmt.Println("c.Info.Version", c.Info.Version)
	// fmt.Println("c.Info.Description.Content", c.Info.Description.Content)
	// fmt.Println("c.Info.Description.Type", c.Info.Description.Type)
	// fmt.Println("c.Info.Description.Version", c.Info.Description.Version)
	// fmt.Println("Postman connection info ==== ======================== =====================end\n\n\n\n\n")

	colletionName := stringutils.RemoveSpecialChars(stringutils.RemoveMultipleSpaces(c.Info.Name))

	for _, c := range app.collectionsModel.List() {
		if strings.EqualFold(c.Name, colletionName) {
			colletionName = fmt.Sprintf("%s_%s", colletionName, stringutils.RandomString(6))
		}
	}

	desc := fmt.Sprintf("%s %s", c.Info.Name, c.Info.Version)

	collection := &models.Collection{
		Name: colletionName,
		Desc: desc,
	}
	messageList = append(messageList, fmt.Sprintf("Info: creating collection %s", collection.Name))

	app.collectionsModel.Save(collection)
	for _, item := range c.Items {

		messages := app.ProcessPostmanItem(item, collection, variabels, currentUser)
		messageList = append(messageList, messages...)

	}

	return messageList
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func (app *application) ProcessPostmanItem(item *postman.Items, collection *models.Collection, variables map[string]string, currentUser *models.User) []string {
	messageList := make([]string, 0)

	// fmt.Println("Postman item Variables", item.Variables)
	// fmt.Println("Postman item Events", item.Events)
	// fmt.Println("Postman item ProtocolProfileBehavior", item.ProtocolProfileBehavior)
	// fmt.Println("Postman item ID", item.ID)

	//means this is a itemgroup
	if len(item.Items) > 0 {
		colletionName := stringutils.RemoveSpecialChars(stringutils.RemoveMultipleSpaces(item.Name))
		colletionName = fmt.Sprintf("%s_%s", collection.Name, colletionName)
		for _, c := range app.collectionsModel.List() {
			if strings.EqualFold(c.Name, colletionName) {
				colletionName = fmt.Sprintf("%s_%s", colletionName, stringutils.RandomString(6))
			}
		}

		collection = &models.Collection{
			Name: colletionName,
			Desc: item.Name,
		}
		app.collectionsModel.Save(collection)
		messageList = append(messageList, fmt.Sprintf("Info: creating collection %s", collection.Name))

	} else {

		epName := stringutils.RemoveSpecialChars(stringutils.RemoveMultipleSpaces(item.Name))
		ep := &models.EndPoint{
			Name:           epName,
			CollectionID:   collection.ID,
			CollectionName: collection.Name,
		}

		messageList = append(messageList, fmt.Sprintf("Info: creating endpoint %s", ep.Name))

		if app.endpoints.DuplicateName(ep) {
			ep.Name = fmt.Sprintf("%s_%s", ep.Name, stringutils.RandomString(6))
			messageList = append(messageList, fmt.Sprintf("Info: duplicate name. Renaming to %s", ep.Name))
		}

		if item.Request != nil {
			ep.Method = string(item.Request.Method)
			ep.ActualURL = ReplaceVariables(item.Request.URL.Raw, variables)

			sampleRequest, requestType := ProcessBody(item.Request.Body)
			ep.SampleRequest = ReplaceVariables(sampleRequest, variables)
			ep.SampleRequestType = requestType

			ep.SampleRequestHeader = ReplaceVariables(UrlEncodeToString(item.Request.Header), variables)
			ep.SampleRequestHeaderType = "JSON"

			respmap := ResponseToEndPointResponse(item.Responses, variables)

			for _, epR := range respmap {
				ep.SetResponse(epR)
			}

			ep.Prepare()

			if ep.Valid() {
				app.endpoints.Save(ep, currentUser.Email)
			} else {
				messageList = append(messageList, fmt.Sprintf("Error: Endpoint errors %s", ep.Name))

				for k, v := range ep.Validator.FieldErrors {
					messageList = append(messageList, fmt.Sprintf("Error: %s %s %s", ep.Name, k, v))

				}

				messageList = append(messageList, fmt.Sprintf("Error: Endpoint is not valid %s", ep.Name))

				fmt.Println("")

			}
		} else {
			messageList = append(messageList, fmt.Sprintf("Warning: Request not defined. Skipping endpoint %s", ep.Name))

		}

	}

	for _, i := range item.Items {
		messages := app.ProcessPostmanItem(i, collection, variables, currentUser)
		messageList = append(messageList, messages...)

	}

	return messageList

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------
func ProcessBody(body *postman.Body) (string, string) {
	if body == nil {
		return "{}", "JSON"
	}

	switch body.Mode {
	case "raw":
		fmt.Println("raw")

		rawType := ""

		if body.Options != nil {
			rawType = strings.ToUpper(body.Options.Raw.Language)

		}

		if rawType == "JSON" || rawType == "XML" {
			return body.Raw, rawType
		}
		return "{}", "JSON"

	case "urlencoded":
		return UrlEncodeToString(body.URLEncoded), "JSON"
	case "formdata":
		//fmt.Println("formdata", body.FormData)
		return UrlEncodeToString(body.FormData), "JSON"

		// case "file":
		// 	return "{}", "JSON"
		// case "graphql":
		// 	return "{}", "JSON"
	}

	return "{}", "JSON"
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func IsList(k any) bool {
	rt := reflect.TypeOf(k)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	case reflect.Array:
		return true

	}
	return false
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func IsMap(k any) bool {
	rt := reflect.TypeOf(k)

	fmt.Println(":: rt.Kind() ", rt.Kind(), reflect.TypeOf(k))
	switch rt.Kind() {
	case reflect.Map:
		return true

	}
	return false
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func ReplaceVariables(source string, variables map[string]string) string {

	for k, v := range variables {
		source = strings.ReplaceAll(source, k, v)
	}

	return source
}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func UrlEncodeToString(source any) string {

	x, err := json.Marshal(source)
	if err != nil {
		return "{}"

	}
	fmt.Println("JSON***********", string(x))

	j := make([]map[string]any, 0)
	err = json.Unmarshal(x, &j)
	if err != nil {
		return "{}"
	}

	finalMap := make(map[string]string)
	for _, i := range j {
		k, found := i["key"]
		if !found {
			continue
		}
		v, found := i["value"]
		if !found {
			continue
		}

		finalMap[k.(string)] = v.(string)
	}

	j2, err := json.MarshalIndent(finalMap, "", "  ")
	if err != nil {
		return "{}"
	}

	return string(j2)

}

// -----------------------------------------------------------------------
//
// -----------------------------------------------------------------------

func ResponseToEndPointResponse(responses []*postman.Response, variables map[string]string) []*models.EndPointResponse {

	epResps := make([]*models.EndPointResponse, 0)

	if responses == nil {
		return epResps
	}

	for _, r := range responses {
		epR := &models.EndPointResponse{}
		epR.HttpCode = r.Code
		epR.Name = r.Name
		epR.Response = "{}"
		epR.ResponseType = "JSON"

		rawType := strings.ToUpper(r.PreviewLanguage)
		if rawType == "JSON" || rawType == "XML" {
			epR.Response = r.Body
			epR.ResponseType = rawType
		}
		if r.Headers != nil {

			epR.ResponseHeader = ReplaceVariables(UrlEncodeToString(r.Headers.Headers), variables)
		}

		epResps = append(epResps, epR)
	}
	return epResps
}
