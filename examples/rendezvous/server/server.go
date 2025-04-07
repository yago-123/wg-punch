package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yago-123/wg-punch/pkg/rendezvous/server"
	"github.com/yago-123/wg-punch/pkg/rendezvous/store"
)

func main() {
	// Setup store (you can swap this with a persistent implementation later)
	rendezvousStore := store.NewMemoryStore()

	// Initialize the server
	srv := server.NewRendezvousServer(rendezvousStore)

	// Start the server
	addr := "0.0.0.0:8080"
	if err := srv.Start(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	log.Printf("Rendezvous server started on %s", addr)

	// Graceful shutdown on SIGINT or SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server gracefully stopped")
}
