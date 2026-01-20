package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"lambda/internal/logger"

	"go.uber.org/zap"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type Config struct {
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime string // e.g., "1h"
}

type ExecutionDetails struct {
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	Command []string `json:"command"`
}

type ResourceDetails struct {
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
}

type Function struct {
	Name      string
	Type      string
	Image     string
	Execution ExecutionDetails
	Resources ResourceDetails
	Env       map[string]string
	TimeoutMS int
}

type DB struct {
	conn   *sql.DB
	driver string
}

func Connect(cfg Config) (*DB, error) {
	connStr := os.Getenv("DB_URL")
	if connStr == "" {
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
	}

	logger.Log.Debug("Connecting to database...", zap.String("host", cfg.Host), zap.String("dbname", cfg.DBName))
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		logger.Log.Error("Failed to open database connection", zap.Error(err))
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Set connection pool settings
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25) // Default
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(25) // Default
	}

	if cfg.ConnMaxLifetime != "" {
		if d, err := time.ParseDuration(cfg.ConnMaxLifetime); err == nil {
			db.SetConnMaxLifetime(d)
		}
	} else {
		db.SetConnMaxLifetime(5 * time.Minute) // Default
	}

	return &DB{conn: db, driver: "postgres"}, nil
}

func ConnectSQLite(path string) (*DB, error) {
	logger.Log.Debug("Connecting to SQLite database...", zap.String("path", path))
	db, err := sql.Open("sqlite", path)
	if err != nil {
		logger.Log.Error("Failed to open SQLite database connection", zap.Error(err))
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{conn: db, driver: "sqlite"}, nil
}

func (db *DB) RunMigrations(migrationsPath string) error {
	logger.Log.Info("Running database migrations...", zap.String("path", migrationsPath), zap.String("driver", db.driver))

	var driver migratedb.Driver
	var err error

	if db.driver == "postgres" {
		driver, err = postgres.WithInstance(db.conn, &postgres.Config{})
	} else if db.driver == "sqlite" {
		driver, err = sqlite.WithInstance(db.conn, &sqlite.Config{})
	}

	if err != nil {
		return fmt.Errorf("could not create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
		db.driver, driver)
	if err != nil {
		return fmt.Errorf("could not create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("could not run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		logger.Log.Info("No database migrations to apply")
	} else {
		logger.Log.Info("Database migrations applied successfully")
	}

	return nil
}

func (db *DB) SaveFunction(f Function) error {
	logger.Log.Debug("Saving function...", zap.String("name", f.Name))
	execData, _ := json.Marshal(f.Execution)
	resData, _ := json.Marshal(f.Resources)
	envData, _ := json.Marshal(f.Env)

	query := `
	INSERT INTO functions (name, type, image, execution, resources, env, timeout_ms)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (name) DO UPDATE SET
		type = EXCLUDED.type,
		image = EXCLUDED.image,
		execution = EXCLUDED.execution,
		resources = EXCLUDED.resources,
		env = EXCLUDED.env,
		timeout_ms = EXCLUDED.timeout_ms;`

	_, err := db.conn.Exec(query, f.Name, f.Type, f.Image, execData, resData, envData, f.TimeoutMS)
	if err != nil {
		logger.Log.Error("Failed to save function", zap.String("name", f.Name), zap.Error(err))
		return err
	}
	return nil
}

func (db *DB) GetFunction(name string) (*Function, error) {
	logger.Log.Debug("Fetching function...", zap.String("name", name))
	query := `SELECT name, type, image, execution, resources, env, timeout_ms FROM functions WHERE name = $1`
	var f Function
	var image sql.NullString
	var execData, resData, envData []byte

	err := db.conn.QueryRow(query, name).Scan(&f.Name, &f.Type, &image, &execData, &resData, &envData, &f.TimeoutMS)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Warn("Function not found", zap.String("name", name))
		} else {
			logger.Log.Error("Failed to get function", zap.String("name", name), zap.Error(err))
		}
		return nil, err
	}
	f.Image = image.String

	json.Unmarshal(execData, &f.Execution)
	json.Unmarshal(resData, &f.Resources)
	json.Unmarshal(envData, &f.Env)

	return &f, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping() error {
	return db.conn.Ping()
}
