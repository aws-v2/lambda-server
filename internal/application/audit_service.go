package application

import (
	"context"
	"lambda/internal/domain"
	"log"
)

type AuditService struct {
	invocationRepo domain.InvocationRepository
}

func NewAuditService(invocationRepo domain.InvocationRepository) *AuditService {
	return &AuditService{
		invocationRepo: invocationRepo,
	}
}

func (s *AuditService) LogInvocation(ctx context.Context, invocation *domain.Invocation) {
	log.Printf("AUDIT: Function=%s, Status=%s, Duration=%dms, User=%s",
		invocation.FunctionName,
		invocation.Status,
		invocation.DurationMs,
		"userId",
	)
}

func (s *AuditService) GetAuditLogs(ctx context.Context, functionID string, limit int) ([]domain.Invocation, error) {
	return s.invocationRepo.ListByFunction(ctx, functionID, limit)
}
