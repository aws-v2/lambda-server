package repository

import (
	"context"
	"database/sql"
	"fmt"
	"lambda/internal/domain"
)

type PostgresFunctionRepository struct {
	db *sql.DB
}

func NewPostgresFunctionRepository(db *sql.DB) *PostgresFunctionRepository {
	return &PostgresFunctionRepository{db: db}
}

func (r *PostgresFunctionRepository) Save(ctx context.Context, function *domain.Function) error {
	query := `
		INSERT INTO functions (id, name, owner_id, runtime, code, status, created_at, updated_at, memory_mb, timeout_ms)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		function.ID,
		function.Name,
		function.OwnerID,
		function.Runtime,
		function.Code,
		function.Status,
		function.CreatedAt,
		function.UpdatedAt,
		function.MemoryMB,
		function.TimeoutMs,
	)
	if err != nil {
		return fmt.Errorf("failed to save function: %w", err)
	}
	return nil
}

func (r *PostgresFunctionRepository) GetByID(ctx context.Context, id string) (*domain.Function, error) {
	query := `
		SELECT id, name, owner_id, runtime, code, status, created_at, updated_at, memory_mb, timeout_ms
		FROM functions WHERE id = $1
	`
	function := &domain.Function{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&function.ID,
		&function.Name,
		&function.OwnerID,
		&function.Runtime,
		&function.Code,
		&function.Status,
		&function.CreatedAt,
		&function.UpdatedAt,
		&function.MemoryMB,
		&function.TimeoutMs,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("function not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get function: %w", err)
	}
	return function, nil
}

func (r *PostgresFunctionRepository) GetByName(ctx context.Context, name, ownerID string) (*domain.Function, error) {
	query := `
		SELECT id, name, owner_id, runtime, code, status, created_at, updated_at, memory_mb, timeout_ms
		FROM functions WHERE name = $1 AND owner_id = $2
	`
	function := &domain.Function{}
	err := r.db.QueryRowContext(ctx, query, name, ownerID).Scan(
		&function.ID,
		&function.Name,
		&function.OwnerID,
		&function.Runtime,
		&function.Code,
		&function.Status,
		&function.CreatedAt,
		&function.UpdatedAt,
		&function.MemoryMB,
		&function.TimeoutMs,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("function not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get function: %w", err)
	}
	return function, nil
}

func (r *PostgresFunctionRepository) ListByOwner(ctx context.Context, ownerID string) ([]domain.Function, error) {
	query := `
		SELECT id, name, owner_id, runtime, code, status, created_at, updated_at, memory_mb, timeout_ms
		FROM functions WHERE owner_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list functions: %w", err)
	}
	defer rows.Close()

	var functions []domain.Function
	for rows.Next() {
		var function domain.Function
		err := rows.Scan(
			&function.ID,
			&function.Name,
			&function.OwnerID,
			&function.Runtime,
			&function.Code,
			&function.Status,
			&function.CreatedAt,
			&function.UpdatedAt,
			&function.MemoryMB,
			&function.TimeoutMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan function: %w", err)
		}
		functions = append(functions, function)
	}

	return functions, nil
}

func (r *PostgresFunctionRepository) Update(ctx context.Context, function *domain.Function) error {
	query := `
		UPDATE functions SET name = $2, runtime = $3, code = $4, status = $5, updated_at = $6, memory_mb = $7, timeout_ms = $8
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query,
		function.ID,
		function.Name,
		function.Runtime,
		function.Code,
		function.Status,
		function.UpdatedAt,
		function.MemoryMB,
		function.TimeoutMs,
	)
	if err != nil {
		return fmt.Errorf("failed to update function: %w", err)
	}
	return nil
}

func (r *PostgresFunctionRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM functions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete function: %w", err)
	}
	return nil
}
