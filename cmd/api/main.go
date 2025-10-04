package main

import (
	"log"
	"net/http"

	"github.com/EyalShahaf/temporal-seats/internal/config"
	httptransport "github.com/EyalShahaf/temporal-seats/internal/transport/http"
	"go.temporal.io/sdk/client"
)

func main() {
	// 1. Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalf("unable to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	cfg := config.Load()
	router := httptransport.NewRouter(cfg, temporalClient)

	log.Println("API server starting on :8080")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
