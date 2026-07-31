package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/internal/validator"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) ConditionGroupHandlers(router *chi.Mux) {
	router.Route("/conditiongroups/{endpointid}", func(r chi.Router) {
		r.Use(app.RequireAuthentication)

		r.Use(app.EndPointOwnership)
		r.Use(noSurf)

		r.Get("/", app.ConditionGroupList)

		r.Get("/add", app.ConditionGroupUpdate)
		r.Post("/add", app.ConditionGroupUpdatePost)

		r.Get("/update/{paramid}", app.ConditionGroupUpdate)
		r.Post("/update/{paramid}", app.ConditionGroupUpdatePost)

		r.Get("/delete/{objectid}", app.ConditionGroupDelete)
		r.Post("/delete", app.ConditionGroupDeleteConfirm)

		r.Get("/parms/{responseid}/{groupid}", app.GetConditionGParam)
		r.Get("/parms/{responseid}", app.GetConditionGParam)

	})

}

// ------------------------------------------------------
// add new endpoint
// ------------------------------------------------------
func (app *application) GetConditionGParam(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "endpointid")
	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	responseid := chi.URLParam(r, "responseid")

	paramid := chi.URLParam(r, "groupid")
	cgParams := models.BuildConditionGroupParameter(endpoint, responseid)

	if paramid == "" {
		//cgParams = models.BuildConditionGroupParameter(endpoint, 200)
	} else {
		conditionGroup, err := app.conditionGroup.Get(paramid)

		if err != nil {
			cgParams = models.BuildConditionGroupParameter(endpoint, responseid)

		} else {
			conditionGroup.Initialize(endpoint, responseid)

			cgParams = conditionGroup.ConditionGroupParameters
		}

	}

	app.writeJSON(w, http.StatusOK, cgParams, nil)
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) ConditionGroupList(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "endpointid")
	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	data := app.newTemplateData(r)
	data.ConditionGroups = app.conditionGroup.ListById(endpoint.ID)

	data.EndPoint = endpoint
	app.render(w, r, http.StatusOK, "condition_group_list.tmpl", data)

}

// ------------------------------------------------------
// Delete servet
// ------------------------------------------------------
func (app *application) ConditionGroupDelete(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "endpointid")
	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	cgID := chi.URLParam(r, "objectid")

	cg, err := app.conditionGroup.Get(cgID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	data := app.newTemplateData(r)
	data.EndPoint = endpoint
	data.ConditionGroup = cg

	app.render(w, r, http.StatusOK, "condition_group_delete.tmpl", data)

}

// ------------------------------------------------------
// Delete servet
// ------------------------------------------------------
func (app *application) ConditionGroupDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	app.invalidateEndPointCache()

	endpointID := chi.URLParam(r, "endpointid")

	err := r.ParseForm()
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	objectid := r.PostForm.Get("objectid")

	err = app.conditionGroup.Delete(objectid)
	if err != nil {

		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error deleting Condition group: %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}
	app.sessionManager.Put(r.Context(), "flash", "Deleted sucessfully")

	http.Redirect(w, r, fmt.Sprintf("/conditiongroups/%s", endpointID), http.StatusSeeOther)

}

// ------------------------------------------------------
// add new endpoint
// ------------------------------------------------------
func (app *application) ConditionGroupUpdate(w http.ResponseWriter, r *http.Request) {
	endpointID := chi.URLParam(r, "endpointid")
	endpoint, err := app.endpoints.Get(endpointID)
	if err != nil {
		app.Http404(w, r)
		return
	}

	paramKeys := make([]string, 0)
	requestParams := app.requestParams.ListById(endpoint.ID)

	for _, requestParam := range requestParams {
		paramKeys = append(paramKeys, fmt.Sprintf("REQUEST[%s]: %s", requestParam.DefaultDatatype, requestParam.Key))
	}

	randomFuncKeys := models.GetRandonFunctiolist()
	paramKeys = append(paramKeys, randomFuncKeys...)

	data := app.newTemplateData(r)

	data.RequestParamAutoComplateList = paramKeys

	data.Conditions = app.condition.ListById(endpoint.ID)
	paramid := chi.URLParam(r, "paramid")

	if paramid == "" {
		conditionGroup := &models.ConditionGroup{
			ResponseID: endpoint.GetDefaultResponseID().ID,
		}
		data.Form = conditionGroup.Initialize(endpoint, endpoint.GetDefaultResponseID().ID)

	} else {
		conditionGroup, err := app.conditionGroup.Get(paramid)

		if err != nil {
			app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error updating endpoint: %s", err.Error()))
			app.goBack(w, r, http.StatusBadRequest)
			return
		}
		data.Form = conditionGroup.Initialize(endpoint, conditionGroup.ResponseID)
	}

	data.EndPoint = endpoint

	app.render(w, r, http.StatusOK, "condition_group_add.tmpl", data)

}

// ------------------------------------------------------
// add new endpoint
// ------------------------------------------------------
func (app *application) ConditionGroupUpdatePost(w http.ResponseWriter, r *http.Request) {
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

	var conditionGroup models.ConditionGroup

	err = app.formDecoder.Decode(&conditionGroup, r.PostForm)
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("002 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	conditionGroup.CheckField(validator.NotBlank(conditionGroup.Name), "name", "This field cannot be blank")
	conditionGroup.CheckField(!app.conditionGroup.DuplicateName(&conditionGroup, *endpoint), "name", "Duplicate Name")

	invalidMappedParams := false
	for _, mappedParam := range conditionGroup.ConditionGroupParameters {

		if mappedParam == nil { //TODO ==> check why nil
			continue
		}

		// Only literals can be type-checked here. REQUEST[...] and *RANDOM:...
		// are resolved per request, so the expression itself never looks like a
		// number or a bool even when the value it produces does.
		if strings.TrimSpace(mappedParam.AssgineValue) != "" && !models.IsDynamicAssignment(mappedParam.AssgineValue) {
			mappedParam.CheckField(validator.MustBeOfType(mappedParam.AssgineValue, mappedParam.ResponseVariableDatatype), "assignvalue", fmt.Sprintf("Make sure value is compatible with %s", mappedParam.ResponseVariableDatatype))
			if !mappedParam.Valid() {
				invalidMappedParams = true
			}
		}
	}

	//http.StatusSeeOther

	if !conditionGroup.Valid() || invalidMappedParams {
		data := app.newTemplateData(r)
		// Re-initialise against the response the user actually picked. An empty
		// id matches no response, which made RemoveInvalidResponseParam discard
		// every mapping and hand back a form with all the user's field
		// assignments gone.
		conditionGroup.Initialize(endpoint, conditionGroup.ResponseID)
		data.Form = conditionGroup
		data.EndPoint = endpoint
		data.Conditions = app.condition.ListById(endpoint.ID)

		app.sessionManager.Put(r.Context(), "error", "Please fix error(s) and resubmit")

		app.render(w, r, http.StatusUnprocessableEntity, "condition_group_add.tmpl", data)
		return
	}

	conditionGroup.EndpointID = endpoint.ID
	_, err = app.conditionGroup.Save(&conditionGroup)
	if err != nil {
		app.serverError500(w, r, err)
		return
	}

	app.invalidateEndPointCache()

	app.sessionManager.Put(r.Context(), "flash", fmt.Sprintf("Condition Group %s added sucessfully", conditionGroup.Name))

	http.Redirect(w, r, fmt.Sprintf("/conditiongroups/%s", endpointID), http.StatusSeeOther)
}
