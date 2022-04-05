# Basic Instrumented GORM HTTP Server

This is a basic example of how to instrument a GORM HTTP application with the New Relic Go agent. In order to run this, you will need go 1.15+ installed. This applicaiton requires a mySQL server to be running to work, and must be given a correct conneciton string on startup to work. It is currently configured to connect to a mySQL server running in a container on the host network, but can be modified to connect to any mySQL server. If it does not exist, it will create a database with a table named "Product" with the following schema: 

|Key|Type|
|---|----|
|GORM Model | Unique ID |
| Code | varchar |
| Name | varchar |
| Price | int |

To launch the application, run:

 ```shell
 go mod tidy
 go run main.go
 ```

 Once it is running, you can interract with the web UI at localhost:8000 in your browser. In the application's logs, it a line will be printed saying "Reporting to: ...." This is where you can access the data collected on your application by New Relic.