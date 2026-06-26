// Command server runs the seeds-go todo HTTP API on :8080.
package main

import (
	"log"
	"net/http"

	seeds "example.com/seeds-go"
)

func main() {
	store := seeds.NewStore()
	srv := seeds.NewServer(store)
	log.Println("seeds-go listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", srv))
}
