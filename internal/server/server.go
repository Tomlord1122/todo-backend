package server

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"todo-backend/internal/database"
	"todo-backend/internal/service"
)

type Server struct {
	port        int
	todoService service.TodoService
	db          database.Service
}

func NewServer(todoService service.TodoService, dbService database.Service) *http.Server {
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Printf("Warning: Invalid PORT environment variable '%s'. Using default 8080. Error: %v", portStr, err)
		port = 8080
	}

	appServer := &Server{
		port:        port,
		todoService: todoService,
		db:          dbService,
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", appServer.port),
		Handler:      appServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
