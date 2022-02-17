// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/driver/sqlite"
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

func NewApp() *App {
	// initialize sqlite server with GORM
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// Migrate the schema
	db.AutoMigrate(&Product{})

	return &App{db: db}
}

func badRequest(w http.ResponseWriter, message string) {
	w.WriteHeader(http.StatusBadRequest)
	strError := strconv.Itoa(http.StatusBadRequest)
	response := fmt.Sprintf("%s - %s", strError, message)
	w.Write([]byte(response))
}

func index(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	p := "." + r.URL.Path
	if p == "./" {
		p = "./index.html"
	}
	http.ServeFile(w, r, p)
}

func (app *App) get(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Printf("error parsing form during scan: %v\n", err)
		return
	}

	code := r.Form.Get("code")
	name := strings.ToLower(r.Form.Get("name"))

	var product Product
	var response string
	if code != "" {
		app.db.First(&product, "code = ?", code)
	} else if name != "" {
		app.db.First(&product, "name = ?", name)
	} else {
		response = "either name or code must be provided for get"
	}

	if response != "" {
		badRequest(w, response)
		log.Printf("Bad Request - %s\n", response)
		return
	}

	response = fmt.Sprintf("%s,%s: $%s", product.Name, product.Code, strconv.Itoa(product.Price))
	io.WriteString(w, response)
}

func (app *App) add(w http.ResponseWriter, r *http.Request) {
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
		log.Fatal(err)
	}

	app.db.Create(&Product{
		Code:  code,
		Name:  name,
		Price: intPrice,
	})

	response := fmt.Sprintf("Added Product: {Code: %s, Name: %s, Price: %s}", code, name, price)
	io.WriteString(w, response)
}

func main() {
	app := NewApp()

	// HTTP handlers
	http.HandleFunc("/", index)
	http.HandleFunc("/add", app.add)
	http.HandleFunc("/get", app.get)

	http.ListenAndServe(":8000", nil)
}
