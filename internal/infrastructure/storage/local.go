package storage

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"lambda/internal/logger"

	"go.uber.org/zap"
)

type Storage struct {
	BaseDir string
}

func NewStorage(baseDir string) *Storage {
	return &Storage{BaseDir: baseDir}
}

func (s *Storage) SaveFunctionBinary(name string, reader io.Reader) (string, error) {
	// Create directory: ./storage/functions/<name>/
	funcDir := filepath.Join(s.BaseDir, "functions", name)
	logger.Log.Debug("Creating function directory", zap.String("path", funcDir))
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		logger.Log.Error("Failed to create function directory", zap.String("path", funcDir), zap.Error(err))
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	dstPath := filepath.Join(funcDir, "handler")
	logger.Log.Debug("Saving function binary", zap.String("path", dstPath))
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		logger.Log.Error("Failed to open binary file for writing", zap.String("path", dstPath), zap.Error(err))
		return "", fmt.Errorf("failed to open destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, reader); err != nil {
		return "", fmt.Errorf("failed to save binary: %w", err)
	}

	// Return the absolute path
	absPath, err := filepath.Abs(dstPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	return absPath, nil
}

func (s *Storage) SaveFunctionZip(name string, reader io.ReaderAt, size int64) (string, error) {
	// Create directory: ./storage/functions/<name>/
	funcDir := filepath.Join(s.BaseDir, "functions", name)
	
	// Clean the directory if it exists to avoid stale files
	logger.Log.Debug("Cleaning function directory", zap.String("path", funcDir))
	if err := os.RemoveAll(funcDir); err != nil {
		logger.Log.Warn("Failed to clean directory", zap.String("path", funcDir), zap.Error(err))
	}
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		logger.Log.Error("Failed to create function directory", zap.String("path", funcDir), zap.Error(err))
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return "", fmt.Errorf("failed to create zip reader: %w", err)
	}

	for _, f := range zipReader.File {
		// Zip Slip Vulnerability Protection
		fpath := filepath.Join(funcDir, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(funcDir)+string(os.PathSeparator)) {
			return "", fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return "", err
		}

		dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return "", err
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return "", err
		}

		_, err = io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
		if err != nil {
			return "", err
		}
	}

	// Return the absolute path to the DIRECTORY
	absPath, err := filepath.Abs(funcDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	return absPath, nil
}
