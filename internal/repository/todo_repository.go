package repository

import (
	"todo-backend/internal/domain"

	"gorm.io/gorm"
)

// TodoRepository defines the interface for todo data operations
type TodoRepository interface {
	Create(todo *domain.Todo) error
	FindByID(id uint) (*domain.Todo, error)
	GetAll() ([]domain.Todo, error)
	Update(todo *domain.Todo) error
	Delete(id uint) error
}

// gormTodoRepository implements TodoRepository using GORM
type gormTodoRepository struct {
	db *gorm.DB
}

// NewGormTodoRepository creates a new GORM todo repository
func NewGormTodoRepository(db *gorm.DB) TodoRepository {
	return &gormTodoRepository{db: db}
}

// Create adds a new todo to the database
func (r *gormTodoRepository) Create(todo *domain.Todo) error {
	// GORM's Create method handles inserting the record
	result := r.db.Create(todo)
	return result.Error // Return any error encountered
}

// FindByID retrieves a todo by its ID
func (r *gormTodoRepository) FindByID(id uint) (*domain.Todo, error) {
	var todo domain.Todo
	// GORM's First method finds the first record matching the condition (ID)
	result := r.db.First(&todo, id) // Find by primary key
	if result.Error != nil {
		// Handle potential errors, like gorm.ErrRecordNotFound
		return nil, result.Error
	}
	return &todo, nil
}

// GetAll retrieves all todos
func (r *gormTodoRepository) GetAll() ([]domain.Todo, error) {
	var todos []domain.Todo
	// GORM's Find method retrieves all records into the slice
	result := r.db.Find(&todos)
	if result.Error != nil {
		return nil, result.Error
	}
	return todos, nil
}

// Update modifies an existing todo
func (r *gormTodoRepository) Update(todo *domain.Todo) error {
	// GORM's Save method updates all fields or inserts if primary key is zero
	// Or use Updates to update specific fields: r.db.Model(todo).Updates(updatesMap)
	result := r.db.Save(todo)
	return result.Error
}

// Delete removes a todo by its ID
func (r *gormTodoRepository) Delete(id uint) error {
	// GORM's Delete method performs a soft delete if the model includes gorm.Model
	// To permanently delete: r.db.Unscoped().Delete(&domain.Todo{}, id)
	result := r.db.Delete(&domain.Todo{}, id)
	return result.Error
}
