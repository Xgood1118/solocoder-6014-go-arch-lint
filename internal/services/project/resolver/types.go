package resolver

import (
	"context"
	"regexp"

	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/arch"
)

type (
	projectFilesScanner interface {
		Scan(
			ctx context.Context,
			projectDirectory string,
			moduleName string,
			excludePaths []models.ResolvedPath,
			excludeFileMatchers []*regexp.Regexp,
		) ([]models.ProjectFile, error)
	}

	projectFilesHolder interface {
		HoldProjectFiles(files []models.ProjectFile, components []arch.Component) []models.FileHold
	}

	cacheService interface {
		Read(projectDir string) (models.CacheFile, error)
		ComputeConfigHash(archFilePath string) (string, error)
		IsValid(cache models.CacheFile, currentConfigHash string) bool
		ToProjectFiles(cache models.CacheFile) []models.ProjectFile
	}
)
