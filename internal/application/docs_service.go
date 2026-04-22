package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lambda/internal/utils/logger"

	"go.uber.org/zap"
)

// =====================
// Models (match frontend)
// =====================

type DocItem struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type DocCategory struct {
	Title string    `json:"title"`
	Items []DocItem `json:"items"`
}

type DocManifest struct {
	Service    string        `json:"service"`
	Version    string        `json:"version,omitempty"`
	Categories []DocCategory `json:"categories"`
}

type Metadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	LastUpdated string   `json:"lastUpdated"`
	Tags        []string `json:"tags"`
}

type DocResponse struct {
	Metadata Metadata `json:"metadata"`
	Content  string   `json:"content"`
}

// =====================
// Service
// =====================

type DocsService struct {
	basePath string // e.g. "./docs"
	log      *zap.Logger
}

func NewDocsService(basePath string) *DocsService {
	return &DocsService{
		basePath: basePath,
		log: logger.Log.With(
			zap.String(logger.F.Domain, "docs"),
		),
	}
}

// =====================
// Public API
// =====================

func (s *DocsService) GetManifest(internal bool) (*DocManifest, error) {
	scope := s.getScope(internal)
	path := filepath.Join(s.basePath, scope, "manifest.json")

	s.log.Debug("reading manifest",
		zap.String("path", path),
		zap.String("scope", scope),
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.log.Warn("manifest not found",
				zap.String(logger.F.ErrorKind, "manifest_not_found"),
				zap.String("path", path),
				zap.String("scope", scope),
			)
			return nil, fmt.Errorf("manifest not found at %q — check DOCS_PATH and folder structure", path)
		}
		s.log.Error("failed to read manifest",
			zap.String(logger.F.ErrorKind, "manifest_read_error"),
			zap.String("path", path),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to read manifest at %q: %w", path, err)
	}

	var manifest DocManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		s.log.Error("manifest JSON invalid",
			zap.String(logger.F.ErrorKind, "manifest_parse_error"),
			zap.String("path", path),
			zap.Error(err),
		)
		return nil, fmt.Errorf("invalid manifest JSON at %q: %w", path, err)
	}

	s.log.Debug("manifest loaded",
		zap.String("service", manifest.Service),
		zap.String("version", manifest.Version),
		zap.Int("category_count", len(manifest.Categories)),
	)

	return &manifest, nil
}

// GetDoc loads a markdown file and parses frontmatter.
func (s *DocsService) GetDoc(slug string, internal bool) (*DocResponse, error) {
	scope := s.getScope(internal)

	if !isValidSlug(slug) {
		s.log.Warn("invalid slug rejected",
			zap.String(logger.F.ErrorKind, "invalid_slug"),
			zap.String("slug", slug),
			zap.String("scope", scope),
		)
		return nil, errors.New("invalid slug")
	}

	path := filepath.Join(s.basePath, scope, slug+".md")

	s.log.Debug("reading doc",
		zap.String("slug", slug),
		zap.String("path", path),
		zap.String("scope", scope),
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.log.Warn("doc not found",
				zap.String(logger.F.ErrorKind, "doc_not_found"),
				zap.String("slug", slug),
				zap.String("path", path),
				zap.String("scope", scope),
			)
			return nil, errors.New("not found")
		}
		s.log.Error("failed to read doc",
			zap.String(logger.F.ErrorKind, "doc_read_error"),
			zap.String("slug", slug),
			zap.String("path", path),
			zap.Error(err),
		)
		return nil, errors.New("not found")
	}

	meta, content := parseMarkdownWithFrontmatter(string(data))

	s.log.Debug("doc loaded",
		zap.String("slug", slug),
		zap.String("title", meta.Title),
		zap.Strings("tags", meta.Tags),
		zap.Int("content_bytes", len(content)),
	)

	return &DocResponse{
		Metadata: meta,
		Content:  content,
	}, nil
}

// =====================
// Helpers
// =====================

func (s *DocsService) getScope(internal bool) string {
	if internal {
		return "internal"
	}
	return "public"
}

// isValidSlug prevents path traversal attacks.
func isValidSlug(slug string) bool {
	if slug == "" {
		return false
	}
	if strings.Contains(slug, "..") || strings.Contains(slug, "/") || strings.Contains(slug, "\\") {
		return false
	}
	return true
}

// =====================
// Markdown Parser
// =====================

func parseMarkdownWithFrontmatter(input string) (Metadata, string) {
	var meta Metadata

	parts := strings.SplitN(input, "---", 3)

	if len(parts) < 3 {
		meta.LastUpdated = time.Now().Format("2006-01-02")
		return meta, strings.TrimSpace(input)
	}

	rawMeta := parts[1]
	content := parts[2]

	for _, line := range strings.Split(rawMeta, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "title:"):
			meta.Title = cleanValue(line, "title:")
		case strings.HasPrefix(line, "description:"):
			meta.Description = cleanValue(line, "description:")
		case strings.HasPrefix(line, "icon:"):
			meta.Icon = cleanValue(line, "icon:")
		case strings.HasPrefix(line, "tags:"):
			meta.Tags = parseTags(cleanValue(line, "tags:"))
		}
	}

	meta.LastUpdated = time.Now().Format("2006-01-02")

	return meta, strings.TrimSpace(content)
}

func cleanValue(line, prefix string) string {
	val := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	return strings.Trim(val, `"`)
}

func parseTags(input string) []string {
	input = strings.Trim(input, "[]")
	var tags []string
	for _, t := range strings.Split(input, ",") {
		if tag := strings.TrimSpace(strings.Trim(t, `"`)); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}