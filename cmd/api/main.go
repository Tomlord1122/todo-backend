package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/Tomlord1122/todo-backend/internal/database"
	"github.com/Tomlord1122/todo-backend/internal/domain" // Import domain for potential AutoMigrate
	"github.com/Tomlord1122/todo-backend/internal/repository"
	"github.com/Tomlord1122/todo-backend/internal/server"
	"github.com/Tomlord1122/todo-backend/internal/service"

	_ "github.com/joho/godotenv/autoload" // Keep if loading .env for PORT or DB
)

func gracefulShutdown(apiServer *http.Server, dbService database.Service, done chan bool) {
	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Listen for the interrupt signal.
	<-ctx.Done()

	log.Println("Shutting down gracefully, press Ctrl+C again to force")
	stop() // Allow Ctrl+C to force shutdown

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctxTimeout); err != nil {
		log.Printf("Server forced to shutdown with error: %v", err)
	}

	// Attempt to close the database connection pool gracefully
	if dbService != nil {
		log.Println("Closing database connection pool...")
		if err := dbService.Close(); err != nil {
			log.Printf("Error closing database connection pool: %v", err)
		} else {
			log.Println("Database connection pool closed.")
		}
	}

	log.Println("Server exiting")

	// Notify the main goroutine that the shutdown is complete
	done <- true
}

func main() {
	// 1. Initialize Database (using the GORM version)
	dbService := database.New()

	gormDB := dbService.GetDB() // Get the *gorm.DB instance

	// Optional: Auto-migrate schema (use cautiously in production)
	// Run this only during development or via a separate migration command
	log.Println("Running database auto-migration (dev only!)...")
	err := gormDB.AutoMigrate(&domain.Todo{}) // Add other models here
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	log.Println("Database auto-migration complete.")

	// 2. Initialize Repositories
	todoRepo := repository.NewGormTodoRepository(gormDB)

	// 3. Initialize Services
	todoService := service.NewTodoService(todoRepo)

	// 4. Initialize Server/Router, passing dependencies
	// NewServer now expects both todoService and dbService
	chiServer := server.NewServer(todoService, dbService)

	// Create a done channel to signal when the shutdown is complete
	done := make(chan bool, 1)

	// Run graceful shutdown in a separate goroutine
	// Pass the *http.Server instance directly and the dbService for closing
	go gracefulShutdown(chiServer, dbService, done)

	// Log the actual address the server is listening on
	log.Printf("Starting server on %s", chiServer.Addr)
	err = chiServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) { // Use errors.Is for checking
		log.Fatalf("HTTP server ListenAndServe error: %v", err) // Use log.Fatalf
	}

	// Wait for the graceful shutdown to complete
	<-done
	log.Println("Graceful shutdown complete.")
}
