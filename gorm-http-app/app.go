// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// mysql database hosted in local container for this example
// podman run --name mysql -p 3306:3306 -e MYSQL_ALLOW_EMPTY_PASSWORD=true -e MYSQL_DATABASE="product" mysql

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	// Import newrelic database driver as custom driver
	// GORM will automatically use this driver as its mysql driver
	// https://gorm.io/docs/connecting_to_the_database.html#Customize-Driver
	_ "github.com/newrelic/go-agent/v3/integrations/nrmysql"

	"github.com/newrelic/go-agent/v3/newrelic"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type App struct {
	db *gorm.DB
}

type Product struct {
	gorm.Model
	Code  string
	Name  string
	Price int
}

const (
	BackendError = "backend error"
)

// handler for formatting and sending bad request messages
func errorResponse(w http.ResponseWriter, txn *newrelic.Transaction, errorNumber int, clientError, internalError string) {
	defer txn.StartSegment("errorResponse").End()

	// log error locally
	log.Println(internalError)

	// send http error to client
	// because our app sets the response number header to an error
	// the Go agent will automatically detect it as an error
	w.WriteHeader(errorNumber)
	strError := strconv.Itoa(errorNumber)
	response := fmt.Sprintf("%s - %s", strError, clientError)
	w.Write([]byte(response))
}

// handler for formatting and sending ok request messages
func okResponse(w http.ResponseWriter, txn *newrelic.Transaction, message string) {
	defer txn.StartSegment("okResponse").End()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

// API endpoing for the root of the application
// Serves a static HTTP file
func Index(w http.ResponseWriter, r *http.Request) {
	txn := newrelic.FromContext(r.Context())
	defer txn.StartSegment("Index").End()

	p := "." + r.URL.Path
	if p == "./" {
		p = "./index.html"
	}
	http.ServeFile(w, r, p)
}

// API endpoint for the /get pattern
// gets a single Product from the database by either Name or Code
func (app *App) Get(w http.ResponseWriter, r *http.Request) {
	txn := newrelic.FromContext(r.Context())

	// polulate r.Form
	err := r.ParseForm()
	if err != nil {
		internalError := fmt.Sprintf("error parsing form during GET operation: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, BackendError, internalError)
		return
	}

	// get arguments from http Form
	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))

	// lookup product based on arguments
	var product Product
	ctx := newrelic.NewContext(context.Background(), txn)
	gormdb := app.db.WithContext(ctx)
	if code != "" {
		err = gormdb.First(&product, "code = ?", code).Error
	} else if name != "" {
		err = gormdb.First(&product, "name = ?", name).Error
	} else {
		msg := fmt.Sprintf("bad request: either name or code must be provided for get")
		errorResponse(w, txn, http.StatusBadRequest, msg, msg)
		return
	}

	if err != nil {
		internalError := fmt.Sprintf("unable to GET product: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, BackendError, internalError)
	} else {
		response := fmt.Sprintf("%s,%s: $%s", product.Name, product.Code, strconv.Itoa(product.Price))
		okResponse(w, txn, response)
	}
}

// API endpoint for the /add pattern
// adds a single entry to the database
func (app *App) Add(w http.ResponseWriter, r *http.Request) {
	txn := newrelic.FromContext(r.Context())

	// Populate r.Form
	err := r.ParseForm()
	if err != nil {
		internalErr := fmt.Sprintf("error parsing form when adding product: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, BackendError, internalErr)
		return
	}

	// Parse arguments from r.Form
	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))
	price := r.Form.Get("price")

	if code == "" || name == "" || price == "" {
		clientError := fmt.Sprintf("bad request: code, name, and price can not be empty")
		errorResponse(w, txn, http.StatusBadRequest, clientError, clientError)
		return
	}

	intPrice, err := strconv.Atoi(price)
	if err != nil {
		internalErr := fmt.Sprintf("error converting %s to an integer: %v", price, err)
		errorResponse(w, txn, http.StatusInternalServerError, BackendError, internalErr)
	}

	// add new product to the database
	ctx := newrelic.NewContext(context.Background(), txn)
	gormdb := app.db.WithContext(ctx)
	err = gormdb.Create(&Product{
		Code:  code,
		Name:  name,
		Price: intPrice,
	}).Error

	if err != nil {
		internalErr := fmt.Sprintf("error creating product: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, BackendError, internalErr)
	}

	response := fmt.Sprintf("Added Product: {Code: %s, Name: %s, Price: %s}", code, name, price)
	okResponse(w, txn, response)
}

func (app *App) Remote(w http.ResponseWriter, r *http.Request) {

}

// NewApp initializes a an app object with a gorm db object and a New Relic Go agent
func NewGORMApp(appName, connectionString string) *App {
	// Wrap database conneciton with GORM
	gormdb, err := gorm.Open(mysql.New(mysql.Config{
		DriverName: "nrmysql",
		DSN:        connectionString,
	}), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// Migrate the schema
	gormdb.AutoMigrate(&Product{})

	return &App{db: gormdb}
}

func main() {
	appName := "gorm-web-app"

	// initialize new relic go aganet app
	goAgent, err := newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigFromEnvironment(),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize database connection
	app := NewGORMApp(appName, "root@/product?charset=utf8mb4&parseTime=True&loc=Local")

	// HTTP handlers
	http.HandleFunc(newrelic.WrapHandleFunc(goAgent, "/", Index))
	http.HandleFunc(newrelic.WrapHandleFunc(goAgent, "/add", app.Add))
	http.HandleFunc(newrelic.WrapHandleFunc(goAgent, "/get", app.Get))

	http.ListenAndServe(":8000", nil)
}
