package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/internal/validator"
)

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) UsersHandlers(router *chi.Mux) {
	router.Route("/users", func(r chi.Router) {
		r.Use(app.RequireSuperAdmin)

		// CSRF
		r.Use(noSurf)
		r.Get("/", app.userList)
		r.Get("/add", app.userAdd)
		r.Post("/add", app.userAdd)
		r.Get("/edit/{userid}", app.userAdd)
		r.Post("/edit/{userid}", app.userAdd)

		r.Get("/delete/{userid}", app.UserDelete)
		r.Post("/delete", app.UserDeleteConfirm)
	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) userList(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "Display a HTML form for signing up a new user...")

	data := app.newTemplateData(r)
	data.Users = app.users.List()
	app.render(w, r, http.StatusOK, "user_list.tmpl", data)
}

// ------------------------------------------------------
//
// ------------------------------------------------------
func (app *application) userAdd(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintln(w, "Display a HTML form for signing up a new user...")

	user := &models.User{}

	userId := chi.URLParam(r, "userid")
	if userId != "" {
		u, err := app.users.Get(userId)
		if err == nil {
			user = u

		}
	}

	if r.Method == http.MethodPost {
		err := app.decodePostForm(r, &user)
		if err != nil {
			app.clientError(w, http.StatusBadRequest, err)
			return
		}

		user.CheckField(validator.NotBlank(user.Name), "name", "This field cannot be blank")

		user.CheckField(validator.NotBlank(user.Email), "email", "This field cannot be blank")
		user.CheckField(validator.Matches(user.Email, validator.EmailRX), "email", "This field must be a valid email address")

		user.CheckField(validator.NotBlank(user.Password), "password", "This field cannot be blank")
		user.CheckField(validator.MinChars(user.Password, 8), "password", "This field must be at least 8 characters long")

		if user.Valid() {
			if user.ID == "" {
				user.MaxAllowedEndpoints = app.maxAllowedEndPointsPerUser
			}
			_ = app.users.Save(user, true)

			nextUrl := safeRedirectTarget(r.URL.Query().Get("next"), "/users")
			http.Redirect(w, r, nextUrl, http.StatusSeeOther)
			return
		}
	}

	// roles := app.rbac.Model.ListRoles()
	// data.RbacRoles = make([]string, 0)

	// for k := range roles {
	// 	data.RbacRoles = append(data.RbacRoles, k)
	// }
	data := app.newTemplateData(r)
	data.Form = user

	app.render(w, r, http.StatusOK, "user_add.tmpl", data)
}

// ------------------------------------------------------
// Delete servet
// ------------------------------------------------------
func (app *application) UserDelete(w http.ResponseWriter, r *http.Request) {
	userid := chi.URLParam(r, "userid")

	user, err := app.users.Get(userid)
	if err != nil {
		app.clientError(w, http.StatusNotFound, err)
		return
	}

	data := app.newTemplateData(r)
	data.User = user

	app.render(w, r, http.StatusOK, "user_delete.tmpl", data)

}

// ------------------------------------------------------
// Delete servet
// ------------------------------------------------------
func (app *application) UserDeleteConfirm(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("001 Error processing form %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}

	userid := r.PostForm.Get("userid")

	err = app.users.Delete(userid)
	if err != nil {

		app.sessionManager.Put(r.Context(), "error", fmt.Sprintf("Error deleting User: %s", err.Error()))
		app.goBack(w, r, http.StatusBadRequest)
		return
	}
	app.sessionManager.Put(r.Context(), "flash", "Deleted sucessfully")

	http.Redirect(w, r, "/users", http.StatusSeeOther)

}
