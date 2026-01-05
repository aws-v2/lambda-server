package repository

import (
	"context"
	"database/sql"
	"fmt"
	"lambda/internal/domain"
)

type PostgresInvocationRepository struct {
	db *sql.DB
}

func NewPostgresInvocationRepository(db *sql.DB) *PostgresInvocationRepository {
	return &PostgresInvocationRepository{db: db}
}

func (r *PostgresInvocationRepository) Save(ctx context.Context, invocation *domain.Invocation) error {
	query := `
		INSERT INTO invocations (id, function_id, function_name, status, payload, result, error, started_at, completed_at, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		invocation.ID,
		invocation.FunctionID,
		invocation.FunctionName,
		invocation.Status,
		invocation.Payload,
		invocation.Result,
		invocation.Error,
		invocation.StartedAt,
		invocation.CompletedAt,
		invocation.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("failed to save invocation: %w", err)
	}
	return nil
}

func (r *PostgresInvocationRepository) GetByID(ctx context.Context, id string) (*domain.Invocation, error) {
	query := `
		SELECT id, function_id, function_name, status, payload, result, error, started_at, completed_at, duration_ms
		FROM invocations WHERE id = $1
	`
	invocation := &domain.Invocation{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&invocation.ID,
		&invocation.FunctionID,
		&invocation.FunctionName,
		&invocation.Status,
		&invocation.Payload,
		&invocation.Result,
		&invocation.Error,
		&invocation.StartedAt,
		&invocation.CompletedAt,
		&invocation.DurationMs,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invocation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get invocation: %w", err)
	}
	return invocation, nil
}

func (r *PostgresInvocationRepository) ListByFunction(ctx context.Context, functionID string, limit int) ([]domain.Invocation, error) {
	query := `
		SELECT id, function_id, function_name, status, payload, result, error, started_at, completed_at, duration_ms
		FROM invocations WHERE function_id = $1 ORDER BY started_at DESC LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, functionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list invocations: %w", err)
	}
	defer rows.Close()

	var invocations []domain.Invocation
	for rows.Next() {
		var invocation domain.Invocation
		err := rows.Scan(
			&invocation.ID,
			&invocation.FunctionID,
			&invocation.FunctionName,
			&invocation.Status,
			&invocation.Payload,
			&invocation.Result,
			&invocation.Error,
			&invocation.StartedAt,
			&invocation.CompletedAt,
			&invocation.DurationMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invocation: %w", err)
		}
		invocations = append(invocations, invocation)
	}

	return invocations, nil
}

func (r *PostgresInvocationRepository) Update(ctx context.Context, invocation *domain.Invocation) error {
	query := `
		UPDATE invocations SET status = $2, result = $3, error = $4, completed_at = $5, duration_ms = $6
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query,
		invocation.ID,
		invocation.Status,
		invocation.Result,
		invocation.Error,
		invocation.CompletedAt,
		invocation.DurationMs,
	)
	if err != nil {
		return fmt.Errorf("failed to update invocation: %w", err)
	}
	return nil
}
