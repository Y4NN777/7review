package main

import (
	"log"

	"github.com/Y4NN777/7review/agent/app"
)

func main() {
	server, err := app.NewServer()
	if err != nil {
		log.Fatal(err)
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
