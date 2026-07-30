package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	"github.com/onlysumitg/GoMockAPI/utils/concurrent"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/acme/autocert"
)

func main() {

	today := time.Now().Local().Format("20060102")

	err := os.MkdirAll("./db", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	err = os.MkdirAll("./lic", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll("./env", os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Initializing....")

	// go run ./cmd/web -port=4002 -host="localhost"
	// go run ./cmd/web -h  ==> help text
	// default value for addr => ":4000"

	// using single var
	// addr := flag.String("addr", ":4000", "HTTP work addess")
	// fmt.Printf("\nStarting servers at port %s", *addr)
	// err := http.ListenAndServe(*addr, getTestRoutes())

	//using struct

	//--------------------------------------- Setup CLI paramters ----------------------------
	var params parameters
	params.Load()

	envPort := os.Getenv("PORT")
	port, err := strconv.Atoi(envPort)
	if err == nil {

		params.port = port
		//log.Println("Using port>>> ", port, params.port)
	}

	// --------------------------------------- Setup database ----------------------------
	db, err := bolt.Open("db/internal.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	logdb, err := bolt.Open(fmt.Sprintf("db/log_%s.db", today), 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer logdb.Close()

	// --------------------------------------- Setup app config and dependency injection ----------------------------
	app := baseAppConfig(params, db, logdb)
	routes := app.routes()
	app.batches()

	//--------------------------------------- Setup websockets ----------------------------

	addr, hostUrl := params.getHttpAddress()

	log.Printf("GoMockAPI is live at  %s \n", hostUrl)

	// this is short cut to create http.Server and  server.ListenAndServe()
	// err := http.ListenAndServe(params.addr, routes)

	app.mainAppServer = &http.Server{
		Addr:     addr,
		Handler:  routes,
		ErrorLog: app.errorLog,
	}

	//  --------------------------------------- Data clean up job----------------------------

	go app.clearLogsSchedular(db)

	//--------------------------------------- Create super user ----------------------------

	go app.CreateSuperUser(params.superuseremail, params.superuserpwd)

	// Construct a tls.config
	//tlsConfig := app.getCertificateToUse()
	if params.https {
		var m *autocert.Manager
		app.mainAppServer.TLSConfig, m = app.getCertificateAndManager()

		// lets encrypt need port 80 to run verification
		if app.useletsencrypt {
			go concurrent.RecoverAndRestart(10, "http server", func() { http.ListenAndServe(":http", m.HTTPHandler(nil)) })
		}
		err = app.mainAppServer.ListenAndServeTLS("", "")

	} else {
		err = app.mainAppServer.ListenAndServe()

	}

	log.Println(err)

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func (app *application) clearLogsSchedular(db *bolt.DB) {
	s := gocron.NewScheduler(time.Local)

	s.Every(1).Day().At("21:30").Do(func() {
		models.DailyDataCleanup(db, app.LogDB)
	})
	s.StartAsync()

	//s.Jobs()

	if app.testMode {
		t := gocron.NewScheduler(time.Local)

		t.Every(1).Day().At("21:30").Do(func() {
			models.DailyDataCleanup_TESTMODE(db)
		})
		t.StartAsync()

	}
}
