package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/onlysumitg/GoMockAPI/internal/validator"
	"github.com/onlysumitg/GoMockAPI/utils/httputils"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) EndPointResponseHandlers(router *chi.Mux) {
	router.Route("/epr/{endpointid}", func(r chi.Router) {
		//r.With(paginate).Get("/", listArticles)
		// CSRF
		r.Use(app.RequireAuthentication)

		r.Use(app.EndPointOwnership)
		r.Use(noSurf)
		r.Get("/", app.ResponseList)

		r.Get("/{paramid}", app.ResponseView)

		r.Get("/add", app.ResponseUpdate)
		r.Post("/add", app.ResponseUpdate)

		r.Get("/update/{paramid}", app.ResponseUpdate)
		r.Post("/update/{paramid}", app.ResponseUpdate)

		r.Get("/delete/{objectid}", app.ResponseDelete)
		r.Post("/delete", app.ResponseDeleteConfirm)

	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) ResponseList(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "endpointid")

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	data := app.newTemplateData(r)
	data.EndPointResponses = endpoint.ResponseMap
	data.EndPoint = endpoint
	app.render(w, r, http.StatusOK, "response_list.tmpl", data)

}

// ------------------------------------------------------
// Delete servet
// ------------------------------------------------------
func (app *application) ResponseDelete(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "endpointid")

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	paramid := chi.URLParam(r, "objectid")

	response := endpoint.GetResponseByID(paramid)
	if response.ID == "" {
		app.Http404(w, r)
		return
	}

	data := app.newTemplateData(r)
	data.EndPoint = endpoint
	data.EndPointResponse = response
	//http.StatusAccepted
	app.render(w, r, http.StatusOK, "response_delete.tmpl", data)

}

// ------------------------------------------------------
// Delete servet
// ------------------------------------------------------
func (app *application) ResponseDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	app.invalidateEndPointCache()

	endpointID := chi.URLParam(r, "endpointid")
	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}
	err = r.ParseForm()
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	objectid := r.PostForm.Get("objectid")
	endpoint.RemoveResponse(objectid)
	_, err = app.endpoints.Save(endpoint, "")
	if err != nil {

		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error deleting Condition: %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}
	app.sessionManager.Put(r.Context(), "flash", "Deleted sucessfully")

	http.Redirect(w, r, fmt.Sprintf("/epr/%s", endpointID), http.StatusSeeOther)

}

// ------------------------------------------------------
// EndPoint details
// ------------------------------------------------------
func (app *application) ResponseView(w http.ResponseWriter, r *http.Request) {

	endpointID := chi.URLParam(r, "endpointid")

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}
	paramid := chi.URLParam(r, "paramid")

	data := app.newTemplateData(r)

	data.EndPointResponse = endpoint.GetResponseByID(paramid)

	data.EndPoint = endpoint
	app.render(w, r, http.StatusOK, "response_view.tmpl", data)

}

// ------------------------------------------------------
// add new endpoint
// ------------------------------------------------------
func (app *application) ResponseUpdate(w http.ResponseWriter, r *http.Request) {
	app.invalidateEndPointCache()

	endpointID := chi.URLParam(r, "endpointid")

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	paramid := chi.URLParam(r, "paramid")

	response := endpoint.GetResponseByID(paramid)

	if r.Method == http.MethodPost {
		err := app.decodePostForm(r, &response)
		if err != nil {
			app.clientError(w, http.StatusBadRequest, err)
			return
		}

		response.Name = strings.ToUpper(response.Name)

		response.CheckField(validator.NotBlank(response.Name), "name", "This field cannot be blank")

		// httpcode should be from list
		response.CheckField(validator.MustBeOneOfInt(response.HttpCode, httputils.CodesList...), "httpcode", "Please select a valid value")

		//duplicate http code not allowed
		response.CheckField(!endpoint.IsDuplicateHTTPCodeResponse(response.HttpCode, response), "httpcode", "Already in use")

		response.CheckField(!endpoint.IsDuplicateResponseWithNameDefault(response.HttpCode, response), "name", "Name 'DEFAULT' can be used only once.")

		// valid json/xml : response
		response.CheckField(validator.MustBeFromList(response.ResponseHeaderType, "JSON", "XML"), "headertype", "Valid values are JSON or XML")
		if response.ResponseHeaderType == "JSON" {
			response.CheckField(validator.MustBeJSON(response.ResponseHeader), "header", "Must be a valid JSON")
		}

		if response.ResponseHeaderType == "XML" {
			response.CheckField(validator.MustBeXML(response.ResponseHeader), "header", "Must be a valid XML")
		}
		// valid json/xml : header
		response.CheckField(validator.MustBeFromList(response.ResponseType, "JSON", "XML", "TEXT"), "responsetype", "Valid values are JSON, XML or TEXT")
		if response.ResponseType == "JSON" {
			response.CheckField(validator.MustBeJSON(response.Response), "response", "Must be a valid JSON")
		}

		if response.ResponseType == "XML" {
			response.CheckField(validator.MustBeXML(response.Response), "response", "Must be a valid XML")
		}

		if response.Valid() {
			endpoint.SetResponse(response)
			app.endpoints.Save(endpoint, "")
			http.Redirect(w, r, fmt.Sprintf("/epr/%s", endpointID), http.StatusSeeOther)
			return

		}
	}

	data := app.newTemplateData(r)
	// if paramid == "" {
	// 	data.Form = models.EndPointResponse{}
	// } else {
	// 	response = endpoint.GetResponseByID(paramid)

	// }

	data.Form = response
	data.EndPoint = endpoint

	app.render(w, r, http.StatusOK, "response_add.tmpl", data)

}
