package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fe3dback/go-arch-lint/internal/models"
)

const (
	filePerms = 0o600
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) CacheFilePath(projectDir string) string {
	return filepath.Join(projectDir, models.CacheFileName)
}

func (s *Service) ComputeConfigHash(archFilePath string) (string, error) {
	data, err := os.ReadFile(archFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read arch file for hash: %w", err)
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func (s *Service) Write(projectDir string, cache models.CacheFile) error {
	path := s.CacheFilePath(projectDir)

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	err = os.WriteFile(path, data, filePerms)
	if err != nil {
		return fmt.Errorf("failed to write cache file '%s': %w", path, err)
	}

	return nil
}

func (s *Service) Read(projectDir string) (models.CacheFile, error) {
	path := s.CacheFilePath(projectDir)

	data, err := os.ReadFile(path)
	if err != nil {
		return models.CacheFile{}, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache models.CacheFile
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return models.CacheFile{}, fmt.Errorf("failed to unmarshal cache file: %w", err)
	}

	return cache, nil
}

func (s *Service) IsValid(cache models.CacheFile, currentConfigHash string) bool {
	if cache.SchemaVersion != models.CacheSchemaVersion {
		return false
	}

	if cache.ConfigHash != currentConfigHash {
		return false
	}

	return true
}

func (s *Service) ToProjectFiles(cache models.CacheFile) []models.ProjectFile {
	files := make([]models.ProjectFile, 0, len(cache.ProjectFiles))

	for _, cf := range cache.ProjectFiles {
		imports := make([]models.ResolvedImport, 0, len(cf.Imports))
		for _, ci := range cf.Imports {
			imports = append(imports, models.ResolvedImport{
				Name:       ci.Name,
				ImportType: models.ImportType(ci.ImportType),
			})
		}

		files = append(files, models.ProjectFile{
			Path:    cf.Path,
			Imports: imports,
		})
	}

	return files
}

func (s *Service) FromProjectFiles(files []models.ProjectFile, configHash string, moduleName string) models.CacheFile {
	cacheFiles := make([]models.CacheProjectFile, 0, len(files))

	for _, f := range files {
		cacheImports := make([]models.CacheResolvedImport, 0, len(f.Imports))
		for _, imp := range f.Imports {
			cacheImports = append(cacheImports, models.CacheResolvedImport{
				Name:       imp.Name,
				ImportType: uint8(imp.ImportType),
			})
		}

		cacheFiles = append(cacheFiles, models.CacheProjectFile{
			Path:    f.Path,
			Imports: cacheImports,
		})
	}

	return models.CacheFile{
		SchemaVersion: models.CacheSchemaVersion,
		ConfigHash:    configHash,
		ModuleName:    moduleName,
		ProjectFiles:  cacheFiles,
	}
}
