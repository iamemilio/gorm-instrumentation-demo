// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/newrelic/go-agent/v3/newrelic"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type App struct {
	App *newrelic.Application
	db  *gorm.DB
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

func NewApp(appName string) *App {
	// initialize new relic go aganet app
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName(appName),
		newrelic.ConfigFromEnvironment(),
		newrelic.ConfigDebugLogger(os.Stdout),
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// initialize sqlite server connection with GORM
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// Migrate the schema
	db.AutoMigrate(&Product{})

	return &App{db: db, App: app}
}

// handler for formatting and sending bad request messages
func errorResponse(w http.ResponseWriter, txn *newrelic.Transaction, errorNumber int, clientError, internalError error) {
	// Observe Http response using new relic segment
	defer txn.StartSegment("okResponse").End()

	// log error locally
	log.Println(internalError)

	// log error with go agent
	txn.NoticeError(internalError)

	// send http error to client
	w.WriteHeader(errorNumber)
	strError := strconv.Itoa(errorNumber)
	response := fmt.Sprintf("%s - %s", strError, clientError)
	w.Write([]byte(response))
}

// handler for formatting and sending ok request messages
func okResponse(w http.ResponseWriter, txn *newrelic.Transaction, message string) {
	// Observe Http response using new relic segment
	defer txn.StartSegment("okResponse").End()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

// API endpoing for the root of the application
// Serves a static HTTP file
func (app *App) Index(w http.ResponseWriter, r *http.Request) {
	// Observe serving of Index using new relic segment
	txn := newrelic.FromContext(r.Context())
	defer txn.StartSegment("Index").End()

	p := "." + r.URL.Path
	if p == "./" {
		p = "./index.html"
	}
	http.ServeFile(w, r, p)
}

// a helper function to execute GET database transactions
// gets the first Product that meets the provided condition
func (app *App) getProduct(txn *newrelic.Transaction, condition, value string) (Product, error) {
	// trace the getProduct function with a newRelic Segment
	// create a new relic context to pass to gorm to allow
	// the go agent to observe the database transactions
	defer txn.StartSegment("getProduct").End()
	ctx := newrelic.NewContext(context.Background(), txn)

	var product Product
	gormdb := app.db.WithContext(ctx)
	tx := gormdb.First(&product, condition, value)
	return product, tx.Error
}

// API endpoint for the /get pattern
// gets a single Product from the database by either Name or Code
func (app *App) Get(w http.ResponseWriter, r *http.Request) {
	// Observe serving of Index using new relic segment
	txn := newrelic.FromContext(r.Context())
	defer txn.StartSegment("Get").End()

	// polulate r.Form
	err := r.ParseForm()
	if err != nil {
		clientError := fmt.Errorf(BackendError)
		internalError := fmt.Errorf("error parsing form during GET operation: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, clientError, internalError)
		return
	}

	// get arguments from http Form
	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))

	// lookup product based on arguments
	var product Product
	if code != "" {
		product, err = app.getProduct(txn, "code = ?", code)
	} else if name != "" {
		product, err = app.getProduct(txn, "name = ?", name)
	} else {
		msg := fmt.Errorf("bad request: either name or code must be provided for get")
		errorResponse(w, txn, http.StatusBadRequest, msg, msg)
		return
	}

	if err != nil {
		clientError := fmt.Errorf(BackendError)
		internalError := fmt.Errorf("unable to GET product: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, clientError, internalError)
	} else {
		response := fmt.Sprintf("%s,%s: $%s", product.Name, product.Code, strconv.Itoa(product.Price))
		okResponse(w, txn, response)
	}
}

// a helper function to execute the database create transaction
func (app *App) createProduct(txn *newrelic.Transaction, code, name string, price int) error {
	// trace the createProduct function with a newRelic Segment
	// create a new relic context to pass to gorm to allow
	// the go agent to observe the database transactions
	defer txn.StartSegment("getProduct").End()
	ctx := newrelic.NewContext(context.Background(), txn)

	gormdb := app.db.WithContext(ctx)
	tx := gormdb.Create(&Product{
		Code:  code,
		Name:  name,
		Price: price,
	})
	return tx.Error
}

// API endpoint for the /add pattern
// adds a single entry to the database
func (app *App) Add(w http.ResponseWriter, r *http.Request) {
	// Observe serving the Add handler using new relic segment
	txn := newrelic.FromContext(r.Context())
	defer txn.StartSegment("Get").End()

	// Populate r.Form
	err := r.ParseForm()
	if err != nil {
		clientError := fmt.Errorf(BackendError)
		internalErr := fmt.Errorf("error parsing form when adding product: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, clientError, internalErr)
		return
	}

	// Parse arguments from r.Form
	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))
	price := r.Form.Get("price")

	if code == "" || name == "" || price == "" {
		clientError := fmt.Errorf("bad request: code, name, and price can not be empty")
		errorResponse(w, txn, http.StatusBadRequest, clientError, clientError)
		return
	}

	intPrice, err := strconv.Atoi(price)
	if err != nil {
		clientError := fmt.Errorf(BackendError)
		internalErr := fmt.Errorf("error converting %s to an integer: %v", price, err)
		errorResponse(w, txn, http.StatusInternalServerError, clientError, internalErr)
	}

	// add new product to the database
	err = app.createProduct(txn, code, name, intPrice)
	if err != nil {
		clientError := fmt.Errorf(BackendError)
		internalErr := fmt.Errorf("error creating product: %v", err)
		errorResponse(w, txn, http.StatusInternalServerError, clientError, internalErr)
	}

	response := fmt.Sprintf("Added Product: {Code: %s, Name: %s, Price: %s}", code, name, price)
	okResponse(w, txn, response)
}

// a helper function that wrapps the http.handleFunc in a newrelic wrapper
func (app *App) Handle(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(newrelic.WrapHandleFunc(app.App, pattern, handler))
}

func main() {
	app := NewApp("gorm-demo")

	// HTTP handlers
	app.Handle("/", app.Index)
	app.Handle("/add", app.Add)
	app.Handle("/get", app.Get)

	http.ListenAndServe(":8000", nil)
}
