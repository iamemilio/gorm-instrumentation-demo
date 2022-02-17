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
func errorResponse(w http.ResponseWriter, errorNumber int, message string) {
	w.WriteHeader(errorNumber)
	strError := strconv.Itoa(errorNumber)
	response := fmt.Sprintf("%s - %s", strError, message)
	w.Write([]byte(response))
}

// handler for formatting and sending ok request messages
func okResponse(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

// API endpoing for the root of the application
// Serves a static HTTP file
func (app *App) Index(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	p := "." + r.URL.Path
	if p == "./" {
		p = "./index.html"
	}
	http.ServeFile(w, r, p)
}

// a helper function to execute GET database transactions
// gets the first Product that meets the provided condition
func (app *App) getProduct(condition, value string) (Product, error) {
	var product Product
	txn := app.App.StartTransaction("sqliteRead")
	ctx := newrelic.NewContext(context.Background(), txn)
	gormdb := app.db.WithContext(ctx)
	tx := gormdb.First(&product, condition, value)
	return product, tx.Error
}

// API endpoint for the /get pattern
// gets a single Product from the database by either Name or Code
func (app *App) Get(w http.ResponseWriter, r *http.Request) {
	var err error
	err = r.ParseForm()
	if err != nil {
		log.Printf("error parsing form during scan: %v\n", err)
		return
	}

	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))

	var product Product
	var response string
	if code != "" {
		product, err = app.getProduct("code = ?", code)
	} else if name != "" {
		product, err = app.getProduct("name = ?", name)
	} else {
		response = "either name or code must be provided for get"
		errorResponse(w, http.StatusBadRequest, response)
		log.Printf("Bad Request - %s\n", response)
		return
	}

	if err != nil {
		log.Printf("error getting product: %s", err)
		errorResponse(w, http.StatusInternalServerError, "backend error: unable to get product")
		return
	}

	response = fmt.Sprintf("%s,%s: $%s", product.Name, product.Code, strconv.Itoa(product.Price))
	okResponse(w, response)
}

// a helper function to execute the database create transaction
func (app *App) createProduct(code, name string, price int) error {
	txn := app.App.StartTransaction("sqliteCreate")
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
	err := r.ParseForm()
	if err != nil {
		log.Printf("error parsing form during scan: %v\n", err)
		return
	}

	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))
	price := r.Form.Get("price")

	if code == "" || name == "" || price == "" {
		log.Println("code, name, and price can not be empty")
		return
	}

	intPrice, err := strconv.Atoi(price)
	if err != nil {
		log.Println(err)
	}

	err = app.createProduct(code, name, intPrice)
	if err != nil {
		log.Printf("product create error: %s\n", err)
	}

	response := fmt.Sprintf("Added Product: {Code: %s, Name: %s, Price: %s}", code, name, price)
	okResponse(w, response)
}

// a helper function that wrapps the http.handleFunc in a newrelic wrapper
func (app *App) wrappedHandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	http.HandleFunc(newrelic.WrapHandleFunc(app.App, pattern, handler))
}

func main() {
	app := NewApp("gorm-demo")

	// HTTP handlers
	app.wrappedHandleFunc("/", app.Index)
	app.wrappedHandleFunc("/add", app.Add)
	app.wrappedHandleFunc("/get", app.Get)

	http.ListenAndServe(":8000", nil)
}
