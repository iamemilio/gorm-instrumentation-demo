package main

import (
	"context"
	"os"

	_ "github.com/newrelic/go-agent/v3/integrations/nrsqlite3"
	"github.com/newrelic/go-agent/v3/newrelic"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	Code  string
	Price uint
}

func main() {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("GORM SQLite App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		panic(err)
	}

	dialector := sqlite.Dialector{
		DriverName: "nrsqlite3",
		DSN:        "test.db",
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&Product{})

	// Create New Relic Transaction for GORM Create
	txn := app.StartTransaction("GORM SQLite Operation")
	ctx := newrelic.NewContext(context.Background(), txn)
	nrdb := db.WithContext(ctx)

	// Create
	nrdb.Create(&Product{Code: "D42", Price: 100})

	// Read
	var product Product
	nrdb.First(&product, 1)                 // find product with integer primary key
	nrdb.First(&product, "code = ?", "D42") // find product with code D42

	// Update - update product's price to 200
	nrdb.Model(&product).Update("Price", 200)
	// Update - update multiple fields
	nrdb.Model(&product).Updates(Product{Price: 200, Code: "F42"}) // non-zero fields
	nrdb.Model(&product).Updates(map[string]interface{}{"Price": 200, "Code": "F42"})

	// Delete - delete product
	nrdb.Delete(&product, 1)

	// End New Relic GORM Create
	txn.End()
}
