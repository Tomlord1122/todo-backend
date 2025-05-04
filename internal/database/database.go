package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"

	// GORM imports
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger" // Optional: for GORM logging
)

// Service interface might need adjustment depending on what you expose
type Service interface {
	Health() map[string]string
	Close() error    // May not be needed or different with GORM connection pool
	GetDB() *gorm.DB // Method to get the GORM DB instance
}

type service struct {
	db *gorm.DB
}

var (
	database   = os.Getenv("BLUEPRINT_DB_DATABASE")
	password   = os.Getenv("BLUEPRINT_DB_PASSWORD")
	username   = os.Getenv("BLUEPRINT_DB_USERNAME")
	port       = os.Getenv("BLUEPRINT_DB_PORT")
	host       = os.Getenv("BLUEPRINT_DB_HOST")
	schema     = os.Getenv("BLUEPRINT_DB_SCHEMA") // Optional, GORM can handle schema in DSN
	dbInstance *service
)

func New() Service {
	if dbInstance != nil {
		return dbInstance
	}

	// Construct DSN for GORM
	// Example DSN: "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable TimeZone=Asia/Shanghai"
	// Note: search_path might be handled differently or within the DSN if supported by the driver
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		host, username, password, database, port)
	// Add schema if needed and supported, e.g., append " search_path=" + schema

	// Configure GORM logger (optional, good for development)
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Info, // Log level (Silent, Error, Warn, Info)
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)

	// Open GORM connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger, // Use the configured logger
		// Add schema config if needed, e.g., NamingStrategy: schema.NamingStrategy{TablePrefix: schema + "."} but requires testing
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Set connection pool settings (important for production)
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)           // Max number of idle connections
	sqlDB.SetMaxOpenConns(100)          // Max number of open connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Max lifetime of a connection

	dbInstance = &service{db: db}
	return dbInstance
}

func (s *service) GetDB() *gorm.DB {
	return s.db
}

// Health check needs to use the underlying sql.DB from GORM
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)
	sqlDB, err := s.db.DB()
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("failed to get underlying DB for health check: %v", err)
		log.Printf("Error getting DB for health check: %v", err)
		return stats
	}

	// Ping the database
	err = sqlDB.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		log.Printf("db down: %v", err) // Use Printf for non-fatal errors during health check
		return stats
	}

	// Database is up, add more statistics
	stats["status"] = "up"
	stats["message"] = "It's healthy"

	// Get database stats (like open connections, in use, idle, etc.)
	dbStats := sqlDB.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	// Evaluate stats (example thresholds)
	if dbStats.OpenConnections > 80 { // Adjust threshold based on MaxOpenConns
		stats["message"] = "The database is experiencing heavy load."
	}

	if dbStats.WaitCount > 1000 {
		stats["message"] = "The database has a high number of wait events, indicating potential bottlenecks."
	}

	// These checks might need tuning based on pool settings
	if dbStats.MaxIdleClosed > int64(dbStats.OpenConnections)/2 && dbStats.OpenConnections > dbStats.Idle {
		stats["message"] = "Many idle connections are being closed, consider revising the connection pool settings (MaxIdleConns, ConnMaxIdleTime)."
	}

	if dbStats.MaxLifetimeClosed > int64(dbStats.OpenConnections)/2 {
		stats["message"] = "Many connections are being closed due to max lifetime, consider increasing ConnMaxLifetime or revising the connection usage pattern."
	}

	return stats
}

// Close might not be strictly necessary to call manually as GORM manages the pool,
// but if you need to explicitly close the underlying pool:
func (s *service) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		log.Printf("Error getting underlying sql.DB for closing: %v", err)
		return err
	}
	log.Printf("Closing connection pool for database: %s", database)
	return sqlDB.Close()
}
