package main

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/boltstore"
	"github.com/alexedwards/scs/v2"
	"github.com/onlysumitg/GoMockAPI/env"
	bolt "go.etcd.io/bbolt"
)

func getSessionManager(db *bolt.DB) *scs.SessionManager {
	// Use the scs.New() function to initialize a new session manager. Then we
	// configure it to use our MySQL database as the session store, and set a
	// lifetime of 12 hours (so that sessions automatically expire 12 hours
	// after first being created).

	sessionManager := scs.New()
	sessionManager.Lifetime = 48 * time.Hour
	sessionManager.Store = boltstore.NewWithCleanupInterval(db, 200*time.Second)
	sessionManager.Cookie.Persist = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// Keep the session cookie off plaintext connections when the app is serving
	// HTTPS. Tied to the same setting that decides how the server listens, so
	// local HTTP development still works.
	sessionManager.Cookie.Secure = env.UseHttps()

	// 	sessionManager.Lifetime = 3 * time.Hour
	// sessionManager.IdleTimeout = 20 * time.Minute
	// sessionManager.Cookie.Name = "session_id"
	// sessionManager.Cookie.Domain = "example.com"
	// sessionManager.Cookie.HttpOnly = true
	// sessionManager.Cookie.Path = "/example/"
	// sessionManager.Cookie.Persist = true
	// sessionManager.Cookie.SameSite = http.SameSiteStrictMode
	// sessionManager.Cookie.Secure = true

	return sessionManager
}
