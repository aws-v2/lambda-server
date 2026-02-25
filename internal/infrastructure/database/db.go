package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
		SSLMode:         "require",
	}
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
	Name        string
	UserID      string
	Type        string
	Image       string
	Execution   ExecutionDetails
	Resources   ResourceDetails
	Env         map[string]string
	TimeoutMS   int
	Description string
}

type LambdaMetric struct {
	FunctionName string
	UserID       string
	DurationMS   int
	Status       string // 'success', 'error'
	ErrorMessage string
	Timestamp    time.Time
}

type TimelinePoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type LambdaMetricsResponse struct {
	Invocations int             `json:"invocations"`
	Duration    float64         `json:"duration"` // Avg duration as float64
	Errors      int             `json:"errors"`
	Timeline    []TimelinePoint `json:"timeline"`
}

type DB struct {
	conn   *sql.DB
	driver string
}

func NewPostgresDB(cfg Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func Connect(cfg Config) (*DB, error) {
	logger.Log.Debug("Connecting to database...", zap.String("host", cfg.Host), zap.String("dbname", cfg.Database))

	db, err := NewPostgresDB(cfg)
	if err != nil {
		return nil, err
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
	INSERT INTO functions (name, user_id, type, image, execution, resources, env, timeout_ms, description)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT (name) DO UPDATE SET
		user_id = EXCLUDED.user_id,
		type = EXCLUDED.type,
		image = EXCLUDED.image,
		execution = EXCLUDED.execution,
		resources = EXCLUDED.resources,
		env = EXCLUDED.env,
		timeout_ms = EXCLUDED.timeout_ms,
		description = EXCLUDED.description;`

	_, err := db.conn.Exec(query, f.Name, f.UserID, f.Type, f.Image, execData, resData, envData, f.TimeoutMS, f.Description)
	if err != nil {
		logger.Log.Error("Failed to save function", zap.String("name", f.Name), zap.Error(err))
		return err
	}
	return nil
}

func (db *DB) GetFunction(name string, userID string) (*Function, error) {
	logger.Log.Debug("Fetching function...", zap.String("name", name), zap.String("userID", userID))
	query := `SELECT name, user_id, type, image, execution, resources, env, timeout_ms, description FROM functions WHERE name = $1 AND user_id = $2`
	var f Function
	var image, uID, desc sql.NullString
	var execData, resData, envData []byte

	err := db.conn.QueryRow(query, name, userID).Scan(&f.Name, &uID, &f.Type, &image, &execData, &resData, &envData, &f.TimeoutMS, &desc)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Warn("Function not found", zap.String("name", name), zap.String("userID", userID))
		} else {
			logger.Log.Error("Failed to get function", zap.String("name", name), zap.Error(err))
		}
		return nil, err
	}
	f.Image = image.String
	f.UserID = uID.String

	json.Unmarshal(execData, &f.Execution)
	json.Unmarshal(resData, &f.Resources)
	json.Unmarshal(envData, &f.Env)
	f.Description = desc.String

	return &f, nil
}

func (db *DB) UpdateFunctionConfig(name string, userID string, memory int, timeout int, description string) error {
	logger.Log.Debug("Updating function config...", zap.String("name", name), zap.String("userID", userID))

	// Get existing function to keep its other resource fields (like CPU)
	fn, err := db.GetFunction(name, userID)
	if err != nil {
		return err
	}

	fn.Resources.Memory = memory
	fn.TimeoutMS = timeout * 1000 // Convert seconds to ms
	fn.Description = description

	resData, _ := json.Marshal(fn.Resources)

	query := `UPDATE functions SET resources = $1, timeout_ms = $2, description = $3 WHERE name = $4 AND user_id = $5`
	_, err = db.conn.Exec(query, resData, fn.TimeoutMS, fn.Description, name, userID)
	return err
}

func (db *DB) RecordMetric(m LambdaMetric) error {
	logger.Log.Debug("Recording lambda metric", zap.String("name", m.FunctionName), zap.String("status", m.Status))
	query := `INSERT INTO lambda_metrics (function_name, user_id, duration_ms, status, error_message) VALUES ($1, $2, $3, $4, $5)`
	_, err := db.conn.Exec(query, m.FunctionName, m.UserID, m.DurationMS, m.Status, m.ErrorMessage)
	if err != nil {
		logger.Log.Error("Failed to record metric", zap.Error(err))
	}
	return err
}

func (db *DB) GetMetrics(name string, userID string) (*LambdaMetricsResponse, error) {
	logger.Log.Debug("Fetching metrics", zap.String("name", name), zap.String("userID", userID))

	// 1. Basic Stats (Last 24h)
	statsQuery := `
		SELECT 
			COUNT(*), 
			COALESCE(AVG(duration_ms), 0), 
			COUNT(*) FILTER (WHERE status = 'error')
		FROM lambda_metrics 
		WHERE function_name = $1 AND user_id = $2 AND timestamp > NOW() - INTERVAL '24 hours'`

	var resp LambdaMetricsResponse
	err := db.conn.QueryRow(statsQuery, name, userID).Scan(&resp.Invocations, &resp.Duration, &resp.Errors)
	if err != nil {
		logger.Log.Error("Failed to scan metrics stats", zap.Error(err))
		return nil, err
	}

	// 2. Timeline (Last 24h, hourly buckets)
	timelineQuery := `
		SELECT 
			TO_CHAR(date_trunc('hour', timestamp), 'HH24:00'),
			COUNT(*)
		FROM lambda_metrics
		WHERE function_name = $1 AND user_id = $2 AND timestamp > NOW() - INTERVAL '24 hours'
		GROUP BY 1
		ORDER BY 1 ASC`

	rows, err := db.conn.Query(timelineQuery, name, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p TimelinePoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			continue
		}
		resp.Timeline = append(resp.Timeline, p)
	}

	// Fill empty buckets if needed (optional for now, let frontend handle or fill on backend)
	return &resp, nil
}

func (db *DB) ListFunctionsByUser(userID string) ([]Function, error) {
	logger.Log.Debug("Listing functions for user...", zap.String("userID", userID))
	query := `SELECT name, user_id, type, image, execution, resources, env, timeout_ms, description FROM functions WHERE user_id = $1`

	rows, err := db.conn.Query(query, userID)
	if err != nil {
		logger.Log.Error("Failed to list functions", zap.String("userID", userID), zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var functions []Function
	for rows.Next() {
		var f Function
		var image, uID, desc sql.NullString
		var execData, resData, envData []byte

		if err := rows.Scan(&f.Name, &uID, &f.Type, &image, &execData, &resData, &envData, &f.TimeoutMS, &desc); err != nil {
			logger.Log.Error("Failed to scan function row", zap.Error(err))
			continue
		}
		f.Image = image.String
		f.UserID = uID.String
		f.Description = desc.String
		json.Unmarshal(execData, &f.Execution)
		json.Unmarshal(resData, &f.Resources)
		json.Unmarshal(envData, &f.Env)
		functions = append(functions, f)
	}

	return functions, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) Ping() error {
	return db.conn.Ping()
}
