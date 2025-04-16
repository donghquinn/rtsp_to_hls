package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"org.donghyuns.com/rtsphls/configs"
	"org.donghyuns.com/rtsphls/lib"
	"org.donghyuns.com/rtsphls/router"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Set up configuration
	configs.SetGlobalConfig()
	configs.SetDatabaseConfig()

	// Create stream manager instance
	streamManager := lib.NewStreamManager()

	// Configure HTTP server with router
	ginRouter := router.Network()
	router.SetupRoutes(ginRouter, streamManager)

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + streamManager.Server.HTTPPort,
		Handler: ginRouter,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %s", streamManager.Server.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown server gracefully
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
