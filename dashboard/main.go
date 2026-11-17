package main

import (
	"log"
	_ "time/tzdata"

	"smart-greenhouse/bootstrap"
	"smart-greenhouse/config"
)

func main() {
	app := bootstrap.NewApp()

	log.Fatal(app.Listen(":" + config.Env("APP_PORT", "3000")))
}
