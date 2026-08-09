package storage

import (
	// "archive/zip"
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	// "strings"
	"time"

	"lambda/internal/infrastructure/event"
	"lambda/internal/utils/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Storage struct {
	BaseDir    string
	natsPrefix string
	natsClient *event.NatsClient
}

func NewStorage(baseDir, natsPrefix string, natsClient *event.NatsClient) *Storage {
	return &Storage{BaseDir: baseDir, natsPrefix: natsPrefix, natsClient: natsClient}
}

type createPresignedURLRequest struct {
	UserID     string    `json:"user_id"`
	GameID     string    `json:"game_id,omitempty"`
	AssetID    string    `json:"asset_id"`
	AssetType  AssetType `json:"asset_type"`  // "game" | "template"
	AssetName  string    `json:"asset_name"`  // this is the nameof the job/game/render job this asset belongs to like kalshi or ruto tracker
	BucketName string    `json:"bucket_name"` // this is the nameof the job/game/render job this asset belongs to like kalshi or ruto tracker
	Key        string    `json:"key"`

	Sha256 string `json:"sha256"`
}

type AssetType string

const (
	AssetTypeGame     AssetType = "game"
	AssetTypeTemplate AssetType = "template"
	AssetTypeAgent    AssetType = "agent"
	AssetTypeScript   AssetType = "script"
	AssetTypeLambda   AssetType = "lambda"
)

type createPresignedURLResponse struct {
	UploadURL string `json:"upload_url"`
}

func (s *Storage) SaveFunctionBinary(ctx *gin.Context, name string, reader io.Reader, functionID string, fileBytes []byte, filesha string) (string, error) {
	userid := ctx.GetString("userID")
 

	uploadPresignUrl := fmt.Sprintf("%s.s3.task.create_presigned_url", s.natsPrefix)

	fmt.Printf("\n------>:files sha**:\n %s",filesha)




	payload := createPresignedURLRequest{
		UserID:     userid,
		AssetID:    functionID,
		AssetType:  AssetTypeLambda,
		AssetName:  name,
		BucketName: "lambdas",
		Key:        name,
		Sha256:     filesha,
		GameID:     functionID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}
	fmt.Printf("\n\n---6--->*:%v\n\n", payload.Sha256)

	fmt.Printf("The sha thatwas presented to us was %s", filesha)

	resp, err := s.natsClient.Request(ctx, uploadPresignUrl, data, time.Duration(time.Second*5))

	var respo createPresignedURLResponse
	error := json.Unmarshal(resp, &respo)

	if error != nil {
		return "", fmt.Errorf("failed to Unmarshal payload: %w", err)
	}

	fmt.Printf("\n------>*:%v \n", respo)

	functionUploadUrl := respo.UploadURL

	// if _, err := io.Copy(dst, reader); err != nil {
	// 	return "", fmt.Errorf("failed to save binary: %w", err)
	// }

	// // Return the absolute path to the DIRECTORY
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get absolute path: %w", err)
	// }
	return functionUploadUrl, nil
}
func CalculateSHA256Bytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (s *Storage) SaveFunctionZip(ctx *gin.Context, name string, reader io.ReaderAt, size int64, functionID string, fileBytes []byte, filesha string) (string, error) {
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
	userid := ctx.GetString("userID")

	payload := createPresignedURLRequest{
		UserID:     userid,
		AssetID:    functionID,
		AssetType:  AssetTypeLambda,
		AssetName:  name,
		BucketName: "lambdas",
		Key:        name,
		Sha256:     filesha,
		GameID:     functionID,
	}

	uploadPresignUrl := fmt.Sprintf("%s.s3.task.create_presigned_url", s.natsPrefix)

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := s.natsClient.Request(ctx, uploadPresignUrl, data, time.Duration(time.Second*5))

	var respo createPresignedURLResponse
	error := json.Unmarshal(resp, &respo)

	if error != nil {
		return "", fmt.Errorf("failed to Unmarshal payload: %w", err)
	}

	fmt.Printf("\n\n---6--->*:%v\n\n", payload.Sha256)

	functionUploadUrl := respo.UploadURL

	// s.natsClient.Request(ctx, uploadPresignUrl)
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

		_, errw := f.Open()
		if errw != nil {
			dstFile.Close()
			return "", err
		}

		// _, err = io.Copy(dstFile, srcFile)
		// srcFile.Close()
		// dstFile.Close()
		// if err != nil {
		// 	return "", err
		// }
	}

	// Return the absolute path to the DIRECTORY
	// absPath, err := filepath.Abs(funcDir)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get absolute path: %w", err)
	// }
	return functionUploadUrl, nil

}

func (s *Storage) ReadFunctionFile(name string, filename string) ([]byte, error) {
	// If filename is empty, default to "handler"
	if filename == "" {
		filename = "handler"
	}

	path := filepath.Join(s.BaseDir, "functions", name, filename)
	logger.Log.Debug("Reading function file", zap.String("path", path))

	return os.ReadFile(path)
}

func (s *Storage) WriteFunctionFile(name string, filename string, content []byte) error {
	// If filename is empty, default to "handler"
	if filename == "" {
		filename = "handler"
	}

	funcDir := filepath.Join(s.BaseDir, "functions", name)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	path := filepath.Join(funcDir, filename)
	logger.Log.Debug("Writing function file", zap.String("path", path))

	return os.WriteFile(path, content, 0755)
}
