package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tomlord1122/todo-backend/internal/domain"
	"github.com/Tomlord1122/todo-backend/internal/repository"

	"gorm.io/gorm"
)

// Input/Output Structs (Data Transfer Objects - DTOs)
// It's often good practice to use DTOs for input/output to decouple
// the service layer from the HTTP layer and the database layer.

// CreateTodoRequest holds the data needed to create a new todo
type CreateTodoRequest struct {
	Title  string `json:"title" validate:"required"`
	UserID uint   `json:"user_id"`
}

// UpdateTodoRequest holds the data for updating an existing todo.
// Using pointers allows distinguishing between a field being omitted
// vs. being set to its zero value (e.g., setting Completed to false).
type UpdateTodoRequest struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

// TodoResponse is the standard representation of a Todo returned by the service.
type TodoResponse struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	UserID    uint   `json:"user_id"` // Include relevant fields
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// --- Service Interface ---

// TodoService defines the operations for managing todos.
// It contains the core business logic
type TodoService interface {
	// CreateTodo handles the business logic for creating a new todo item.
	CreateTodo(ctx context.Context, req CreateTodoRequest) (*TodoResponse, error)

	// GetTodoByID retrieves a single todo item by its ID.
	GetTodoByID(ctx context.Context, id uint) (*TodoResponse, error)

	// GetAllTodos retrieves a list of all todo items.
	// Consider adding filtering/pagination parameters here later.
	GetAllTodos(ctx context.Context) ([]TodoResponse, error)

	// UpdateTodo handles updating an existing todo item.
	UpdateTodo(ctx context.Context, id uint, req UpdateTodoRequest) (*TodoResponse, error)

	// DeleteTodo handles deleting a todo item by its ID.
	DeleteTodo(ctx context.Context, id uint) error
}

// --- Service Implementation ---

// todoService implements the TodoService interface.
// It depends on a TodoRepository to interact with the data layer.
type todoService struct {
	repo repository.TodoRepository // Dependency on the repository interface
}

// NewTodoService creates a new instance of todoService.
// It takes a TodoRepository as a dependency (Dependency Injection).
func NewTodoService(repo repository.TodoRepository) TodoService {
	// We return the interface type, hiding the implementation detail.
	return &todoService{
		repo: repo,
	}
}

// --- Method Implementations ---

// CreateTodo implements the logic to create a new todo.
func (s *todoService) CreateTodo(ctx context.Context, req CreateTodoRequest) (*TodoResponse, error) {
	// 1. Business Logic/Validation (Example: Check for empty title, although often done in handler/validation middleware)
	if req.Title == "" {
		// In a real app, input validation might happen earlier (e.g., in the handler)
		// using a validation library. But some core business rules might live here.
		return nil, errors.New("title cannot be empty")
	}

	// 2. Prepare domain model
	newTodo := &domain.Todo{
		Title:     req.Title,
		Completed: false,      // Default value
		UserID:    req.UserID, // Assign user ID if provided
	}

	// 3. Call Repository to save the new todo
	err := s.repo.Create(newTodo) // Pass the domain model to the repository
	if err != nil {
		// Log the error internally
		fmt.Printf("Error creating todo in repository: %v\n", err)
		// Return a more generic error to the caller (handler)
		return nil, errors.New("failed to create todo item")
	}

	// 4. Convert the created domain model to a response DTO
	response := &TodoResponse{
		ID:        newTodo.ID, // GORM populates the ID after creation
		Title:     newTodo.Title,
		Completed: newTodo.Completed,
		UserID:    newTodo.UserID,
		CreatedAt: newTodo.CreatedAt.Format(time.RFC3339), // Format timestamp
		UpdatedAt: newTodo.UpdatedAt.Format(time.RFC3339), // Format timestamp
	}

	return response, nil
}

// GetTodoByID implements the logic to retrieve a todo by ID.
func (s *todoService) GetTodoByID(ctx context.Context, id uint) (*TodoResponse, error) {
	// 1. Call Repository to find the todo
	todo, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // Check for specific GORM error
			// Return a "not found" error that the handler can interpret (e.g., return HTTP 404)
			return nil, fmt.Errorf("todo with ID %d not found", id) // Or define custom error types
		}
		// Log other unexpected errors
		fmt.Printf("Error fetching todo %d from repository: %v\n", id, err)
		return nil, errors.New("failed to retrieve todo item")
	}

	// 2. Convert domain model to response DTO
	response := &TodoResponse{
		ID:        todo.ID,
		Title:     todo.Title,
		Completed: todo.Completed,
		UserID:    todo.UserID,
		CreatedAt: todo.CreatedAt.Format(time.RFC3339),
		UpdatedAt: todo.UpdatedAt.Format(time.RFC3339),
	}

	return response, nil
}

// GetAllTodos implements the logic to retrieve all todos.
func (s *todoService) GetAllTodos(ctx context.Context) ([]TodoResponse, error) {
	// 1. Call Repository to get all todos
	todos, err := s.repo.GetAll()
	if err != nil {
		fmt.Printf("Error fetching all todos from repository: %v\n", err)
		return nil, errors.New("failed to retrieve todo items")
	}

	// 2. Convert the slice of domain models to a slice of response DTOs
	responses := make([]TodoResponse, 0, len(todos)) // Pre-allocate slice capacity
	for _, todo := range todos {
		responses = append(responses, TodoResponse{
			ID:        todo.ID,
			Title:     todo.Title,
			Completed: todo.Completed,
			UserID:    todo.UserID,
			CreatedAt: todo.CreatedAt.Format(time.RFC3339),
			UpdatedAt: todo.UpdatedAt.Format(time.RFC3339),
		})
	}

	return responses, nil
}

// UpdateTodo implements the logic to update an existing todo.
func (s *todoService) UpdateTodo(ctx context.Context, id uint, req UpdateTodoRequest) (*TodoResponse, error) {
	// 1. Fetch the existing todo to ensure it exists
	existingTodo, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("todo with ID %d not found for update", id)
		}
		fmt.Printf("Error fetching todo %d for update: %v\n", id, err)
		return nil, errors.New("failed to retrieve todo item for update")
	}

	// 2. Apply updates from the request (only if fields are provided in the request)
	updated := false
	if req.Title != nil && *req.Title != "" && *req.Title != existingTodo.Title {
		// Add business logic validation if needed, e.g., length checks
		existingTodo.Title = *req.Title
		updated = true
	}
	if req.Completed != nil && *req.Completed != existingTodo.Completed {
		existingTodo.Completed = *req.Completed
		updated = true
	}

	// 3. If nothing was updated, maybe return early or just proceed
	if !updated {
		// Return the existing data without hitting the DB again
		// Or you could choose to always call Update, GORM might handle it efficiently
		fmt.Printf("No changes detected for todo %d\n", id)
		// We still convert and return the existing one as if updated
		response := &TodoResponse{
			ID:        existingTodo.ID,
			Title:     existingTodo.Title,
			Completed: existingTodo.Completed,
			UserID:    existingTodo.UserID,
			CreatedAt: existingTodo.CreatedAt.Format(time.RFC3339),
			UpdatedAt: existingTodo.UpdatedAt.Format(time.RFC3339), // GORM might update this anyway on Save
		}
		return response, nil
		// Alternatively: return nil, errors.New("no update applied") - depends on desired API behavior
	}

	// 4. Call Repository to save the updated todo
	// Note: GORM's Save updates all fields, including associations if loaded.
	// Use Update or Updates for more targeted updates if needed.
	err = s.repo.Update(existingTodo)
	if err != nil {
		fmt.Printf("Error updating todo %d in repository: %v\n", id, err)
		return nil, errors.New("failed to update todo item")
	}

	// 5. Convert updated domain model to response DTO
	response := &TodoResponse{
		ID:        existingTodo.ID,
		Title:     existingTodo.Title,
		Completed: existingTodo.Completed,
		UserID:    existingTodo.UserID,
		CreatedAt: existingTodo.CreatedAt.Format(time.RFC3339),
		UpdatedAt: existingTodo.UpdatedAt.Format(time.RFC3339), // GORM updates UpdatedAt automatically
	}

	return response, nil
}

// DeleteTodo implements the logic to delete a todo.
func (s *todoService) DeleteTodo(ctx context.Context, id uint) error {
	// 1. (Optional) Check if the record exists first if you want to return a specific "not found" error.
	//    GORM's Delete usually doesn't error if the record doesn't exist, but RowsAffected will be 0.
	_, err := s.repo.FindByID(id) // Check existence
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("todo with ID %d not found for deletion", id)
		}
		fmt.Printf("Error checking existence of todo %d before delete: %v\n", id, err)
		return errors.New("failed to check todo item before deletion")
	}

	// 2. Call Repository to delete the todo
	err = s.repo.Delete(id)
	if err != nil {
		fmt.Printf("Error deleting todo %d from repository: %v\n", id, err)
		return errors.New("failed to delete todo item")
	}

	// Successfully deleted (or soft-deleted by GORM if using gorm.Model)
	return nil
}
