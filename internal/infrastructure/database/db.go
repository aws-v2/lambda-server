package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func Connect(cfg Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Connected to database: %s", cfg.DBName)
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS functions (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			owner_id VARCHAR(255) NOT NULL,
			runtime VARCHAR(50) NOT NULL,
			code TEXT NOT NULL,
			status VARCHAR(50) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			memory_mb INT NOT NULL DEFAULT 128,
			timeout_ms INT NOT NULL DEFAULT 30000,
			UNIQUE(name, owner_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_functions_owner ON functions(owner_id)`,
		`CREATE TABLE IF NOT EXISTS invocations (
			id VARCHAR(255) PRIMARY KEY,
			function_id VARCHAR(255) NOT NULL,
			function_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			payload TEXT,
			result TEXT,
			error TEXT,
			started_at TIMESTAMP NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMP,
			duration_ms BIGINT,
			FOREIGN KEY (function_id) REFERENCES functions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_invocations_function ON invocations(function_id)`,
		`CREATE INDEX IF NOT EXISTS idx_invocations_started ON invocations(started_at DESC)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Println("Database migrations completed successfully")
	return nil
}
