package repository

import (
	"context"
	"database/sql"

	"lambda/internal/domain/dto"
)
type DB struct {
    conn *sql.DB
}
func (d *DB) Conn() *sql.DB {
    return d.conn
}
type LambdaScalingPolicyRepository struct {
	DB *sql.DB
}

func NewLambdaScalingPolicyRepository(db *sql.DB) *LambdaScalingPolicyRepository {
	return &LambdaScalingPolicyRepository{
		DB: db,
	}
}
func (r *LambdaScalingPolicyRepository) Create(ctx context.Context, req dto.LambdaScalingPolicyRequest) error {
	query := `
	INSERT INTO lambda_scaling_policies (
		tenant_id,
		function_id,
		metric_name,
		scale_up_threshold,
		scale_down_threshold,
		max_concurrency_limit,
		min_concurrency_limit,
		scale_step,
		cooldown_seconds
	)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`

	_, err := r.DB.ExecContext(ctx, query,
		req.TenantID,
		req.FunctionID,
		req.MetricName,
		req.ScaleUpThreshold,
		req.ScaleDownThreshold,
		req.MaxConcurrencyLimit,
		req.MinConcurrencyLimit,
		req.ScaleStep,
		req.CooldownSeconds,
	)

	return err
}

func (r *LambdaScalingPolicyRepository) Update(ctx context.Context, req dto.LambdaScalingPolicyRequest) error {
	query := `
	UPDATE lambda_scaling_policies
	SET
		scale_up_threshold = $4,
		scale_down_threshold = $5,
		max_concurrency_limit = $6,
		min_concurrency_limit = $7,
		scale_step = $8,
		cooldown_seconds = $9,
		updated_at = NOW()
	WHERE tenant_id = $1
	  AND function_id = $2
	  AND metric_name = $3
	`

	_, err := r.DB.ExecContext(ctx, query,
		req.TenantID,
		req.FunctionID,
		req.MetricName,
		req.ScaleUpThreshold,
		req.ScaleDownThreshold,
		req.MaxConcurrencyLimit,
		req.MinConcurrencyLimit,
		req.ScaleStep,
		req.CooldownSeconds,
	)

	return err
}
func (r *LambdaScalingPolicyRepository) Delete(ctx context.Context, tenantID, functionID, metricName string) error {
	query := `
	DELETE FROM lambda_scaling_policies
	WHERE tenant_id = $1
	  AND function_id = $2
	  AND metric_name = $3
	`

	_, err := r.DB.ExecContext(ctx, query, tenantID, functionID, metricName)
	return err
}
func (r *LambdaScalingPolicyRepository) List(ctx context.Context, tenantID string) ([]dto.LambdaScalingPolicyRequest, error) {
	query := `
	SELECT
		tenant_id,
		function_id,
		metric_name,
		scale_up_threshold,
		scale_down_threshold,
		max_concurrency_limit,
		min_concurrency_limit,
		scale_step,
		cooldown_seconds
	FROM lambda_scaling_policies
	WHERE tenant_id = $1
	`

	rows, err := r.DB.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []dto.LambdaScalingPolicyRequest

	for rows.Next() {
		var p dto.LambdaScalingPolicyRequest

		err := rows.Scan(
			&p.TenantID,
			&p.FunctionID,
			&p.MetricName,
			&p.ScaleUpThreshold,
			&p.ScaleDownThreshold,
			&p.MaxConcurrencyLimit,
			&p.MinConcurrencyLimit,
			&p.ScaleStep,
			&p.CooldownSeconds,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, p)
	}

	return result, nil
}