package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/EyalShahaf/temporal-seats/internal/activities"
	"github.com/EyalShahaf/temporal-seats/internal/entities/seat"
	"github.com/EyalShahaf/temporal-seats/internal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"google.golang.org/grpc"
)

func main() {
	// Check if we should start workers (integration mode)
	if os.Getenv("INTEGRATION") != "1" {
		log.Println("Skipping worker startup - set INTEGRATION=1 to enable")
		log.Println("This prevents accidental background worker processes during development")
		return
	}

	// Create context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Create client with connection validation
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
		ConnectionOptions: client.ConnectionOptions{
			DialOptions: []grpc.DialOption{grpc.WithBlock()},
		},
	})
	if err != nil {
		log.Fatalf("Temporal server not reachable at localhost:7233: %v", err)
	}
	defer c.Close()

	log.Println("Connected to Temporal server successfully")

	var wg sync.WaitGroup
	wg.Add(2)

	// Order processing worker
	go func() {
		defer wg.Done()
		w := worker.New(c, "order-tq", worker.Options{})
		w.RegisterWorkflow(workflows.OrderOrchestrationWorkflow)
		w.RegisterActivity(activities.ValidatePaymentActivity)
		w.RegisterActivity(activities.ConfirmOrderActivity)
		w.RegisterActivity(activities.FailOrderActivity)
		w.RegisterActivity(activities.SeatSignalActivity)
		log.Println("Starting Order Worker")

		// Graceful shutdown
		go func() {
			<-ctx.Done()
			log.Println("Shutting down Order Worker...")
			w.Stop()
		}()

		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Printf("Order Worker failed: %v", err)
		}
	}()

	// Seat entity worker
	go func() {
		defer wg.Done()
		w := worker.New(c, "seat-tq", worker.Options{})
		w.RegisterWorkflow(seat.SeatEntityWorkflow)
		log.Println("Starting Seat Worker")

		// Graceful shutdown
		go func() {
			<-ctx.Done()
			log.Println("Shutting down Seat Worker...")
			w.Stop()
		}()

		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Printf("Seat Worker failed: %v", err)
		}
	}()

	log.Println("All workers started. Press Ctrl+C to stop gracefully.")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received, stopping workers...")

	// Give workers time to stop gracefully
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout reached, forcing exit")
	}
}
