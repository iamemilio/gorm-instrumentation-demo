# Basic Instrumentation Example Based on GORM Quick Start

This is a basic example of how to instrument GORM with the New Relic Go agent that is based on the (GORM quick start example)[https://gorm.io/docs/#Quick-Start]. In order to run this, you will need go 1.15+ installed. This example uses SQLite, and will create a new database named test.db in the working directory it is run from. The SQLite daemon does not need to be actively listening in your enviornment for this example to work, nor does it need to be installed. In order to run this example, run the following commands:

 ```shell
 go mod tidy
 go run main.go
 ```