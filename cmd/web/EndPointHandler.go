package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/utils/stringutils"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) EndPointListMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		next.ServeHTTP(w, r)
	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) EndPointHandlers(router *chi.Mux) {
	router.Route("/endpoints", func(r chi.Router) {
		r.Use(app.RequireAuthentication)

		//r.With(paginate).Get("/", listArticles)

		// CSRF
		r.Use(noSurf)

		r.Get("/", app.EndPointList)
		r.Get("/add", app.EndPointAdd)
		r.Post("/add", app.EndPointAddPost)

		r.Get("/{endpointid}", app.EndPointView)

		r.Get("/update/{endpointid}", app.EndPointAdd)
		r.Post("/update/{endpointid}", app.EndPointAddPost)

		r.Get("/delete/{endpointid}", app.EndPointDelete)
		r.Post("/delete", app.EndPointDeleteConfirm)

		g1 := r.Group(nil)
		g1.Use(app.EndPointOwnership)
		g1.Get("/logs/{endpointid}", app.Endpointlogs)
		g1.Get("/owners/{endpointid}", app.ownerList)
		g1.Post("/addowners/{endpointid}", app.ownerList)

		g1.Get("/copy/{endpointid}", app.makeCopy)

		// g1.Get("/addowners/{endpointid}", app.ownerAdd)
		// g1.Post("/addowners/{endpointid}", app.ownerAdd)
	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) EndPointList(w http.ResponseWriter, r *http.Request) {

	//fmt.Println("......... rout...", chi.RouteContext(r.Context()).RoutePattern())

	user, err := app.GetUser(r)
	if err != nil {
		app.UnauthorizedError(w, r)
		return
	}

	data := app.newTemplateData(r)
	collectionid := r.URL.Query().Get("cid")

	if collectionid != "" {
		collection, err := app.collectionsModel.Get(collectionid)
		if err == nil {
			data.Collection = collection
		} else {
			collectionid = ""
		}
	}

	if user.IsSuperUser {
		if collectionid == "" {

			data.EndPoints = app.endpoints.List()
		} else {
			data.EndPoints = app.endpoints.ListByCollectionID(collectionid)
		}
	} else {
		for _, id := range user.OwnedEndPoints {
			ep, err := app.endpoints.Get(id)
			if err == nil {
				if collectionid == "" {

					data.EndPoints = append(data.EndPoints, ep)
				} else {
					if ep.CollectionID == collectionid {
						data.EndPoints = append(data.EndPoints, ep)

					}
				}
			}
		}

	}

	nextUrl := r.URL.Query().Get("next") //filters=["color", "price", "brand"]
	if nextUrl == "" {
		nextUrl = "/query"
	}
	data.Next = nextUrl
	app.render(w, r, http.StatusOK, "endpoint_list.tmpl", data)

}

// ------------------------------------------------------
// EndPoint details
// ------------------------------------------------------
func (app *application) EndPointView(w http.ResponseWriter, r *http.Request) {

	endpointID := chi.URLParam(r, "endpointid")

	if !app.UserOwnsEndPoint(w, r, endpointID) {
		return
	}

	data := app.newTemplateData(r)
	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.notFound(w, err)
		return
	}
	data.EndPoint = endpoint
	app.render(w, r, http.StatusOK, "endpoint_view.tmpl", data)

}

// ------------------------------------------------------
// EndPoint details
// ------------------------------------------------------
func (app *application) Endpointlogs(w http.ResponseWriter, r *http.Request) {

	endpointID := chi.URLParam(r, "endpointid")

	if !app.UserOwnsEndPoint(w, r, endpointID) {
		return
	}

	data := app.newTemplateData(r)

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.notFound(w, err)
		return
	}
	data.EndPoint = endpoint
	app.render(w, r, http.StatusOK, "endpoint_logs.tmpl", data)

}

// ------------------------------------------------------
// Delete endpoint
// ------------------------------------------------------
func (app *application) EndPointDelete(w http.ResponseWriter, r *http.Request) {

	endpointID := chi.URLParam(r, "endpointid")

	if !app.UserOwnsEndPoint(w, r, endpointID) {
		return
	}

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error deleting endpoint: %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	data := app.newTemplateData(r)
	data.EndPoint = endpoint

	app.render(w, r, http.StatusOK, "endpoint_delete.tmpl", data)

}

// ------------------------------------------------------
// Delete endpoint
// ------------------------------------------------------
func (app *application) EndPointDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	endpointID := r.PostForm.Get("endpointid")

	if !app.UserOwnsEndPoint(w, r, endpointID) {
		return
	}

	err = app.endpoints.Delete(endpointID)
	if err != nil {

		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error deleting endpoint: %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}
	app.invalidateEndPointCache()

	app.sessionManager.Put(r.Context(), "flash", "EndPoint deleted sucessfully")

	http.Redirect(w, r, "/endpoints", http.StatusSeeOther)

}

// ------------------------------------------------------
// add new endpoint
// ------------------------------------------------------
func (app *application) EndPointAdd(w http.ResponseWriter, r *http.Request) {
	//fmt.Println("......... rout.2..", chi.RouteContext(r.Context()).RoutePattern())
	app.invalidateEndPointCache()

	endpointID := chi.URLParam(r, "endpointid")
	data := app.newTemplateData(r)

	if endpointID == "" {

		collectionid := r.URL.Query().Get("cid")

		if collectionid != "" {
			collection, err := app.collectionsModel.Get(collectionid)
			if err == nil {
				data.Collection = collection
			} else {
				collectionid = ""
			}
		}

		// set form initial values
		data.Form = models.EndPoint{CollectionID: collectionid}

		if _, err := app.CanAddMoreEndpoints(r); err != nil {
			app.sessionManager.Put(r.Context(), "error", err.Error())
			app.goBack(w, r, http.StatusSeeOther)
			return
		}

	} else {

		if !app.UserOwnsEndPoint(w, r, endpointID) {
			return
		}

		endpoint, err := app.endpoints.Get(endpointID)
		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error updating endpoint: %s", err.Error()))
			app.goBack(w, r, http.StatusBadRequest)
			return
		}
		data.Form = endpoint
		data.EndPoint = endpoint
	}

	data.Collections = app.collectionsModel.List()
	app.render(w, r, http.StatusOK, "endpoint_add.tmpl", data)

}

// ----------------------------------------------
//
// ----------------------------------------------
func (app *application) EndPointAddPost(w http.ResponseWriter, r *http.Request) {
	user, err := app.GetUser(r)
	if err != nil {
		app.UnauthorizedError(w, r)
		return
	}

	// Limit the request body size to 4096 bytes
	//r.Body = http.MaxBytesReader(w, r.Body, 4096)

	// r.ParseForm() method to parse the request body. This checks
	// that the request body is well-formed, and then stores the form data in the request’s
	// r.PostForm map.
	err = r.ParseForm()
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	// Use the r.PostForm.Get() method to retrieve the title and content
	// from the r.PostForm map.
	//	title := r.PostForm.Get("title")
	//	content := r.PostForm.Get("content")

	// the r.PostForm map is populated only for POST , PATCH and PUT requests, and contains the
	// form data from the request body.

	// In contrast, the r.Form map is populated for all requests (irrespective of their HTTP method),

	var endpoint models.EndPoint
	// Call the Decode() method of the form decoder, passing in the current
	// request and *a pointer* to our snippetCreateForm struct. This will
	// essentially fill our struct with the relevant values from the HTML form.
	// If there is a problem, we return a 400 Bad Request response to the client.
	err = app.formDecoder.Decode(&endpoint, r.PostForm)
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("002 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	endpoint.Prepare()

	endpoint.CheckField(!app.endpoints.DuplicateName(&endpoint), "name", "Duplicate Name")

	// Use the Valid() method to see if any of the checks failed. If they did,
	// then re-render the template passing in the form in the same way as
	// before.

	if !endpoint.Valid() {
		data := app.newTemplateData(r)
		data.Form = endpoint

		data.EndPoint = &endpoint
		app.sessionManager.Put(r.Context(), "error", "Please fix error(s) and resubmit")
		data.Collections = app.collectionsModel.List()

		app.render(w, r, http.StatusUnprocessableEntity, "endpoint_add.tmpl", data)
		return
	}

	if endpoint.ID != "" {

		if !app.UserOwnsEndPoint(w, r, endpoint.ID) {
			return
		}

		originalEP, err := app.endpoints.Get(endpoint.ID)
		if err == nil {
			endpoint.CreatedBy = originalEP.CreatedBy
			endpoint.CreatedOn = originalEP.CreatedOn
			endpoint.ResponseMap = originalEP.ResponseMap
			endpoint.EndPointCallLog = originalEP.EndPointCallLog

		}

	}

	if _, err := app.CanAddMoreEndpoints(r); endpoint.ID == "" && err != nil {
		app.sessionManager.Put(r.Context(), "error", err.Error())
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	endpoint.CollectionName = "V1"
	collection, err := app.collectionsModel.Get(endpoint.CollectionID)
	if err == nil {
		endpoint.CollectionName = collection.Name
	}

	id, err := app.endpoints.Save(&endpoint, user.Email)
	if err != nil {
		app.serverError500(w, r, err)
		return
	} else {
		user.AssignOwnedEndPoint(id)
		app.users.Save(user, false)

	}

	app.invalidateEndPointCache()

	app.sessionManager.Put(r.Context(), "flash", fmt.Sprintf("EndPoint %s saved sucessfully", endpoint.Name))

	http.Redirect(w, r, fmt.Sprintf("/endpoints/update/%s", id), http.StatusSeeOther)
}

// ------------------------------------------------------
// EndPoint details
// ------------------------------------------------------
func (app *application) ownerList(w http.ResponseWriter, r *http.Request) {

	endpointID := chi.URLParam(r, "endpointid")

	if !app.UserOwnsEndPoint(w, r, endpointID) {
		return
	}

	data := app.newTemplateData(r)

	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.notFound(w, err)
		return
	}

	users := app.users.GetEndPointowners(endpointID)

	data.Users = users
	data.EndPoint = endpoint
	app.render(w, r, http.StatusOK, "endpoint_owner_list.tmpl", data)

}

// ------------------------------------------------------
// EndPoint details
// ------------------------------------------------------
// func (app *application) ownerAdd(w http.ResponseWriter, r *http.Request) {

// 	form := models.AddOwnerForm{}
// 	endpointID := chi.URLParam(r, "endpointid")

// 	endpoint, err := app.endpoints.Get(endpointID)
// 	if err != nil {
// 		app.notFound(w, err)
// 		return
// 	}

// 	if r.Method == http.MethodPost {
// 		err := app.decodePostForm(r, &form)

// 		if err != nil {
// 			app.clientError(w, http.StatusBadRequest, err)
// 			return
// 		}

// 		form.CheckField(validator.NotBlank(form.Email), "email", "This field cannot be blank")
// 		form.CheckField(validator.Matches(form.Email, validator.EmailRX), "email", "This field must be a valid email address")
// 		if form.Valid() {

// 		}
// 	}

// 	users := app.users.GetEndPointowners(endpointID)

// 	data := app.newTemplateData(r)

// 	data.Users = users
// 	data.EndPoint = endpoint
// 	data.Form = form
// 	app.render(w, r, http.StatusOK, "endpoint_owner_add.tmpl", data)

// }

// ------------------------------------------------------
// EndPoint details
// ------------------------------------------------------
func (app *application) makeCopy(w http.ResponseWriter, r *http.Request) {

	endpointID := chi.URLParam(r, "endpointid")

	if _, err := app.CanAddMoreEndpoints(r); err != nil {
		app.sessionManager.Put(r.Context(), "error", err.Error())
		app.goBack(w, r, http.StatusSeeOther)
		return
	}

	if !app.UserOwnsEndPoint(w, r, endpointID) {
		return
	}

	user, err := app.GetUser(r)
	if err != nil {
		app.UnauthorizedError(w, r)
		return
	}

	e, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.notFound(w, err)
		return
	}

	e.ID = ""
	e.Name = fmt.Sprintf("%s_%s", e.Name, stringutils.RandomString(6))
	id, _ := app.endpoints.Save(e, user.Email)
	app.invalidateEndPointCache()

	user.AssignOwnedEndPoint(id)
	app.users.Save(user, false)
	app.sessionManager.Put(r.Context(), "flash", fmt.Sprintf("EndPoint %s copied sucessfully", e.Name))

	http.Redirect(w, r, fmt.Sprintf("/endpoints/update/%s", id), http.StatusSeeOther)

}
