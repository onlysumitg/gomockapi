package main

import (
	"crypto/tls"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/alexedwards/scs/v2"
	"github.com/go-playground/form"
	"github.com/onlysumitg/GoMockAPI/internal/models"
	mail "github.com/xhit/go-simple-mail/v2"
	bolt "go.etcd.io/bbolt"
)

type application struct {
	tlsCertificate *tls.Certificate
	tlsMutex       sync.Mutex

	// endPointMutex guards both endPointCache and invalidEndPointCache.
	endPointMutex        sync.RWMutex
	invalidEndPointCache bool

	errorLog *log.Logger
	infoLog  *log.Logger

	DB *bolt.DB

	LogDB *bolt.DB

	EmailServer *mail.SMTPServer

	templateCache map[string]*template.Template
	endPointCache map[string]*models.EndPoint

	maxAllowedEndPoints        int
	maxAllowedEndPointsPerUser int

	formDecoder      *form.Decoder
	sessionManager   *scs.SessionManager
	users            *models.UserModel
	endpoints        *models.EndPointModel
	requestParams    *models.EndPointRequestParamModel
	responseParams   *models.EndPointResponseParamModel
	condition        *models.ConditionModel
	conditionGroup   *models.ConditionGroupModel
	collectionsModel *models.CollectionModel

	mainAppServer *http.Server

	InProduction bool
	hostURL      string
	domain       string

	useletsencrypt bool

	testMode bool
}

func baseAppConfig(params parameters, db *bolt.DB, logdb *bolt.DB) *application {

	//--------------------------------------- Setup loggers ----------------------------
	infoLog := log.New(os.Stderr, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	//--------------------------------------- Setup form decoder ----------------------------
	formDecoder := form.NewDecoder()

	endPointCache := make(map[string]*models.EndPoint)

	_, hostUrl := params.getHttpAddress()
	//---------------------------------------  final app config ----------------------------
	app := &application{
		errorLog:      errorLog,
		infoLog:       infoLog,
		endPointCache: endPointCache,

		DB:          db,
		LogDB:       logdb,
		EmailServer: models.SetupMailServer(),

		sessionManager: getSessionManager(db),
		formDecoder:    formDecoder,
		users:          &models.UserModel{DB: db},
		endpoints:      &models.EndPointModel{DB: db},
		requestParams:  &models.EndPointRequestParamModel{DB: db},
		responseParams: &models.EndPointResponseParamModel{DB: db},
		condition:      &models.ConditionModel{DB: db},
		conditionGroup: &models.ConditionGroupModel{DB: db},

		collectionsModel: &models.CollectionModel{DB: db},

		hostURL: hostUrl,

		maxAllowedEndPoints:        -1,
		maxAllowedEndPointsPerUser: -1,
		testMode:                   params.testmode,

		domain:         params.domain,
		useletsencrypt: params.useletsencrypt,
	}

	if app.testMode {
		app.maxAllowedEndPoints = 50
		app.maxAllowedEndPointsPerUser = 2

	}

	//--------------------------------------- Setup template cache ----------------------------
	templateCache, err := app.newTemplateCache()
	if err != nil {
		errorLog.Fatal(err)
	}

	app.templateCache = templateCache
	return app

}
