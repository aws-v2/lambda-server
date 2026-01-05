package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type LocalCodeStorage struct {
	basePath string
}

func NewLocalCodeStorage(basePath string) (*LocalCodeStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &LocalCodeStorage{basePath: basePath}, nil
}

func (s *LocalCodeStorage) SaveCode(ctx context.Context, functionID, code string) error {
	filePath := filepath.Join(s.basePath, functionID+".js")
	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		return fmt.Errorf("failed to save code: %w", err)
	}
	return nil
}

func (s *LocalCodeStorage) GetCode(ctx context.Context, functionID string) (string, error) {
	filePath := filepath.Join(s.basePath, functionID+".js")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read code: %w", err)
	}
	return string(data), nil
}

func (s *LocalCodeStorage) DeleteCode(ctx context.Context, functionID string) error {
	filePath := filepath.Join(s.basePath, functionID+".js")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete code: %w", err)
	}
	return nil
}
