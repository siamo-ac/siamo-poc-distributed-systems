// Command server runs the event-sourced ledger on :18080: a bank account
// aggregate with an append-only event store (write side) and a balance
// projection (read side) rebuilt via replay.
package main

import (
	"log"
	"net/http"

	"github.com/siamosystems/siamo-poc-distributed-systems/internal/api"
	"github.com/siamosystems/siamo-poc-distributed-systems/internal/events"
	"github.com/siamosystems/siamo-poc-distributed-systems/internal/read"
	"github.com/siamosystems/siamo-poc-distributed-systems/internal/write"
)

func main() {
	store := events.NewStore()
	projection := read.NewProjection()
	// In a real distributed system the read side would subscribe to the log
	// over a message bus and update asynchronously (eventual consistency).
	// This POC applies events to the projection synchronously via callback
	// so the demo is deterministic; the README calls out the difference.
	writes := write.NewService(store, projection.Apply)
	srv := api.NewServer(writes, projection, store)
	log.Println("[ledger] listening on :18080")
	if err := http.ListenAndServe(":18080", srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
