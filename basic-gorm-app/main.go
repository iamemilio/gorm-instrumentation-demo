package main

import (
	"context"
	"os"
	"time"

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
		newrelic.ConfigAppName("GORM App"),
		newrelic.ConfigLicense(os.Getenv("NEW_RELIC_LICENSE_KEY")),
		newrelic.ConfigDebugLogger(os.Stdout),
		newrelic.ConfigDistributedTracerEnabled(true),
	)
	if err != nil {
		panic(err)
	}

	// Wait for go-agent to connect to avoid data loss
	app.WaitForConnection(5 * time.Second)

	dialector := sqlite.Dialector{
		DriverName: "nrsqlite3",
		DSN:        "test.db",
	}
	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	gormDB.AutoMigrate(&Product{})

	// Create New Relic Transaction to monitor GORM Database
	gormTransactionTrace := app.StartTransaction("GORM Operation")
	gormTransactionContext := newrelic.NewContext(context.Background(), gormTransactionTrace)
	tracedDB := gormDB.WithContext(gormTransactionContext)

	// Create
	tracedDB.Create(&Product{Code: "D42", Price: 100})

	// Read
	var product Product
	tracedDB.First(&product, 1)                 // find product with integer primary key
	tracedDB.First(&product, "code = ?", "D42") // find product with code D42

	// Update - update product's price to 200
	tracedDB.Model(&product).Update("Price", 200)
	// Update - update multiple fields
	tracedDB.Model(&product).Updates(Product{Price: 200, Code: "F42"}) // non-zero fields
	tracedDB.Model(&product).Updates(map[string]interface{}{"Price": 200, "Code": "F42"})

	// Delete - delete product
	tracedDB.Delete(&product, 1)

	// End New Relic transaction trace
	gormTransactionTrace.End()

	// Wait for shut down to ensure data gets flushed
	app.Shutdown(5 * time.Second)
}
