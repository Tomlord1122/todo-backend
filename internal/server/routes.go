package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"todo-backend/internal/service"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", s.HelloWorldHandler)

	r.Get("/health", s.healthHandler)

	r.Route("/todos", func(r chi.Router) {
		r.Post("/", s.createTodoHandler)
		r.Get("/", s.getAllTodosHandler)
		r.Get("/{id}", s.getTodoByIDHandler)
		r.Put("/{id}", s.updateTodoHandler)
		r.Delete("/{id}", s.deleteTodoHandler)
	})

	return r
}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Hello World from Todo Backend!"})
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	healthStats := s.db.Health()
	if status, ok := healthStats["status"]; ok && status == "down" {
		respondWithJSON(w, http.StatusServiceUnavailable, healthStats)
		return
	}
	respondWithJSON(w, http.StatusOK, healthStats)
}

func (s *Server) createTodoHandler(w http.ResponseWriter, r *http.Request) {
	var req service.CreateTodoRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		if errors.As(err, &syntaxError) {
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			respondWithError(w, http.StatusBadRequest, msg)
		} else if errors.Is(err, io.ErrUnexpectedEOF) {
			msg := "Request body contains badly-formed JSON"
			respondWithError(w, http.StatusBadRequest, msg)
		} else if errors.As(err, &unmarshalTypeError) {
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			respondWithError(w, http.StatusBadRequest, msg)
		} else if strings.HasPrefix(err.Error(), "json: unknown field ") {
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			respondWithError(w, http.StatusBadRequest, msg)
		} else if errors.Is(err, io.EOF) {
			msg := "Request body must not be empty"
			respondWithError(w, http.StatusBadRequest, msg)
		} else {
			log.Printf("Error decoding create todo request: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Error processing request")
		}
		return
	}

	todoResp, err := s.todoService.CreateTodo(r.Context(), req)
	if err != nil {
		if err.Error() == "title cannot be empty" {
			respondWithError(w, http.StatusBadRequest, err.Error())
		} else {
			log.Printf("Error calling CreateTodo service: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to create todo")
		}
		return
	}

	respondWithJSON(w, http.StatusCreated, todoResp)
}

func (s *Server) getAllTodosHandler(w http.ResponseWriter, r *http.Request) {
	todos, err := s.todoService.GetAllTodos(r.Context())
	if err != nil {
		log.Printf("Error calling GetAllTodos service: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve todos")
		return
	}

	respondWithJSON(w, http.StatusOK, todos)
}

func (s *Server) getTodoByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid todo ID provided")
		return
	}

	todo, err := s.todoService.GetTodoByID(r.Context(), uint(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			log.Printf("Error calling GetTodoByID service: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to retrieve todo")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, todo)
}

func (s *Server) updateTodoHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid todo ID provided")
		return
	}

	var req service.UpdateTodoRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&req)
	if err != nil {
		log.Printf("Error decoding update todo request: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	updatedTodo, err := s.todoService.UpdateTodo(r.Context(), uint(id), req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			log.Printf("Error calling UpdateTodo service: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to update todo")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, updatedTodo)
}

func (s *Server) deleteTodoHandler(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		respondWithError(w, http.StatusBadRequest, "Invalid todo ID provided")
		return
	}

	err = s.todoService.DeleteTodo(r.Context(), uint(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			log.Printf("Error calling DeleteTodo service: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Failed to delete todo")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"Internal server error preparing response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}
