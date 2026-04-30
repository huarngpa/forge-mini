package main

import (
	"log"
	"net/http"

	"forge-mini/internal/app"
	"forge-mini/internal/httpapi"
)

func main() {
	container, err := app.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	defer container.Close()
	server := httpapi.NewServer(container)

	log.Println("forge-mini listening on :8080")
	if err := http.ListenAndServe(":8080", server.Routes()); err != nil {
		log.Fatal(err)
	}
}
