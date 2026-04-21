package application

import (
	"context"
	"database/sql"
	"errors"

	"lambda/internal/domain/dto"
	"lambda/internal/infrastructure/repository"
)

type DB struct {
    conn *sql.DB
}
type LambdaScalingPolicyService struct {
	Repo *repository.LambdaScalingPolicyRepository
}
func NewLambdaScalingPolicyService(repo *repository.LambdaScalingPolicyRepository) *LambdaScalingPolicyService {
	return &LambdaScalingPolicyService{Repo: repo}
}
func (s *LambdaScalingPolicyService) Create(ctx context.Context, req dto.LambdaScalingPolicyRequest) (*dto.LambdaScalingPolicyRequest, error) {
	// basic validation
	if req.FunctionID == "" || req.TenantID == "" || req.MetricName == "" {
		return nil, ErrInvalidRequest("missing required fields")
	}

	err := s.Repo.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func ErrInvalidRequest(s string) error {
return  errors.New("invalid request")

	
}

func (s *LambdaScalingPolicyService) Update(ctx context.Context, req dto.LambdaScalingPolicyRequest) (*dto.LambdaScalingPolicyRequest, error) {
	err := s.Repo.Update(ctx, req)
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (s *LambdaScalingPolicyService) Delete(ctx context.Context, req dto.LambdaScalingPolicyRequest) error {
	return s.Repo.Delete(ctx, req.TenantID, req.FunctionID, req.MetricName)
}

func (s *LambdaScalingPolicyService) List(ctx context.Context, tenantID string) ([]dto.LambdaScalingPolicyRequest, error) {
	return s.Repo.List(ctx, tenantID)
}