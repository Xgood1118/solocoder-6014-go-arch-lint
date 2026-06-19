package cache

import (
	"context"
	"fmt"
	"path"
	"regexp"

	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/arch"
	"github.com/fe3dback/go-arch-lint/internal/models/common"
)

type (
	Operation struct {
		projectInfoAssembler projectInfoAssembler
		specAssembler        specAssembler
		projectFilesScanner  projectFilesScanner
		cacheService         cacheService
	}

	projectInfoAssembler interface {
		ProjectInfo(rootDirectory string, archFilePath string) (common.Project, error)
	}

	specAssembler interface {
		Assemble(prj common.Project) (arch.Spec, error)
	}

	projectFilesScanner interface {
		Scan(
			ctx context.Context,
			projectDirectory string,
			moduleName string,
			excludePaths []models.ResolvedPath,
			excludeFileMatchers []*regexp.Regexp,
		) ([]models.ProjectFile, error)
	}

	cacheService interface {
		CacheFilePath(projectDir string) string
		ComputeConfigHash(archFilePath string) (string, error)
		Write(projectDir string, cache models.CacheFile) error
		FromProjectFiles(files []models.ProjectFile, configHash string, moduleName string) models.CacheFile
	}
)

func NewOperation(
	projectInfoAssembler projectInfoAssembler,
	specAssembler specAssembler,
	projectFilesScanner projectFilesScanner,
	cacheService cacheService,
) *Operation {
	return &Operation{
		projectInfoAssembler: projectInfoAssembler,
		specAssembler:        specAssembler,
		projectFilesScanner:  projectFilesScanner,
		cacheService:         cacheService,
	}
}

func (o *Operation) Behave(ctx context.Context, in models.CmdCacheIn) (models.CmdCacheOut, error) {
	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		return models.CmdCacheOut{}, fmt.Errorf("failed to assemble project info: %w", err)
	}

	spec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return models.CmdCacheOut{}, fmt.Errorf("failed to assemble spec: %w", err)
	}

	configHash, err := o.cacheService.ComputeConfigHash(projectInfo.GoArchFilePath)
	if err != nil {
		return models.CmdCacheOut{}, fmt.Errorf("failed to compute config hash: %w", err)
	}

	scanDirectory := path.Clean(fmt.Sprintf("%s/%s",
		spec.RootDirectory.Value,
		spec.WorkingDirectory.Value,
	))

	projectFiles, err := o.projectFilesScanner.Scan(
		ctx,
		scanDirectory,
		spec.ModuleName.Value,
		refPathToList(spec.Exclude),
		refRegExpToList(spec.ExcludeFilesMatcher),
	)
	if err != nil {
		return models.CmdCacheOut{}, fmt.Errorf("failed to scan project files: %w", err)
	}

	cacheData := o.cacheService.FromProjectFiles(projectFiles, configHash, spec.ModuleName.Value)

	err = o.cacheService.Write(projectInfo.Directory, cacheData)
	if err != nil {
		return models.CmdCacheOut{}, fmt.Errorf("failed to write cache: %w", err)
	}

	return models.CmdCacheOut{
		ProjectDirectory: projectInfo.Directory,
		ModuleName:       spec.ModuleName.Value,
		CacheFile:        o.cacheService.CacheFilePath(projectInfo.Directory),
		FilesCached:      len(projectFiles),
	}, nil
}

func refPathToList(refs []common.Referable[models.ResolvedPath]) []models.ResolvedPath {
	result := make([]models.ResolvedPath, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.Value)
	}
	return result
}

func refRegExpToList(refs []common.Referable[*regexp.Regexp]) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref.Value)
	}
	return result
}
