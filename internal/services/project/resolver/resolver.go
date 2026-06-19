package resolver

import (
	"context"
	"fmt"
	"path"

	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/arch"
)

type Resolver struct {
	projectFilesScanner projectFilesScanner
	projectFilesHolder  projectFilesHolder
	cacheService        cacheService
}

func NewResolver(
	projectFilesScanner projectFilesScanner,
	projectFilesHolder projectFilesHolder,
	cacheService cacheService,
) *Resolver {
	return &Resolver{
		projectFilesScanner: projectFilesScanner,
		projectFilesHolder:  projectFilesHolder,
		cacheService:        cacheService,
	}
}

func (r *Resolver) ProjectFiles(ctx context.Context, spec arch.Spec) ([]models.FileHold, error) {
	if r.cacheService != nil {
		projectFiles, cacheHit, err := r.tryCache(spec)
		if err != nil {
			return nil, fmt.Errorf("cache read failed: %w", err)
		}

		if cacheHit {
			return r.projectFilesHolder.HoldProjectFiles(projectFiles, spec.Components), nil
		}
	}

	scanDirectory := path.Clean(fmt.Sprintf("%s/%s",
		spec.RootDirectory.Value,
		spec.WorkingDirectory.Value,
	))

	projectFiles, err := r.projectFilesScanner.Scan(
		ctx,
		scanDirectory,
		spec.ModuleName.Value,
		refPathToList(spec.Exclude),
		refRegExpToList(spec.ExcludeFilesMatcher),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve project files: %w", err)
	}

	holdFiles := r.projectFilesHolder.HoldProjectFiles(projectFiles, spec.Components)
	return holdFiles, nil
}

func (r *Resolver) tryCache(spec arch.Spec) ([]models.ProjectFile, bool, error) {
	cache, err := r.cacheService.Read(spec.RootDirectory.Value)
	if err != nil {
		return nil, false, nil
	}

	configHash, err := r.cacheService.ComputeConfigHash(spec.GoArchFilePath)
	if err != nil {
		return nil, false, nil
	}

	if !r.cacheService.IsValid(cache, configHash) {
		return nil, false, nil
	}

	projectFiles := r.cacheService.ToProjectFiles(cache)
	return projectFiles, true, nil
}
