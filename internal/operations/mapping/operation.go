package mapping

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/arch"
)

type Operation struct {
	specAssembler        specAssembler
	projectFilesResolver projectFilesResolver
	projectInfoAssembler projectInfoAssembler
}

func NewOperation(
	specAssembler specAssembler,
	projectFilesResolver projectFilesResolver,
	projectInfoAssembler projectInfoAssembler,
) *Operation {
	return &Operation{
		specAssembler:        specAssembler,
		projectFilesResolver: projectFilesResolver,
		projectInfoAssembler: projectInfoAssembler,
	}
}

func (o *Operation) Behave(ctx context.Context, in models.CmdMappingIn) (models.CmdMappingOut, error) {
	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		return models.CmdMappingOut{}, fmt.Errorf("failed to assemble project info: %w", err)
	}

	spec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return models.CmdMappingOut{}, fmt.Errorf("failed to assemble spec: %w", err)
	}

	projectFiles, err := o.projectFilesResolver.ProjectFiles(ctx, spec)
	if err != nil {
		return models.CmdMappingOut{}, fmt.Errorf("failed to resolve project files: %w", err)
	}

	if in.FilterGlob != "" {
		projectFiles = applyFilter(projectFiles, in.FilterGlob)
	}

	return models.CmdMappingOut{
		ProjectDirectory: spec.RootDirectory.Value,
		ModuleName:       spec.ModuleName.Value,
		MappingGrouped:   assembleMappingByComponent(spec, projectFiles),
		MappingList:      assembleMappingByFile(projectFiles),
		Scheme:           in.Scheme,
		FilterGlob:       in.FilterGlob,
	}, nil
}

func applyFilter(files []models.FileHold, filterGlob string) []models.FileHold {
	matcher := newGlobMatcher(filterGlob)
	filtered := make([]models.FileHold, 0, len(files))

	for _, f := range files {
		if matcher.match(f.File.Path) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

type globMatcher struct {
	pattern *regexp.Regexp
}

func newGlobMatcher(glob string) globMatcher {
	escaped := regexp.QuoteMeta(glob)
	escaped = strings.ReplaceAll(escaped, `\*\*`, ".*")
	escaped = strings.ReplaceAll(escaped, `\*`, "[^/]+")
	pattern := "^" + escaped + "$"

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		compiled = regexp.MustCompile(".*")
	}

	return globMatcher{pattern: compiled}
}

func (m globMatcher) match(path string) bool {
	return m.pattern.MatchString(path)
}

func assembleMappingByComponent(
	spec arch.Spec,
	projectFiles []models.FileHold,
) []models.CmdMappingOutGrouped {
	tmp := make(map[string]*models.CmdMappingOutGrouped)

	for _, projectFile := range projectFiles {
		componentName := componentName(projectFile.ComponentID)
		if _, exist := tmp[componentName]; !exist {
			tmp[componentName] = &models.CmdMappingOutGrouped{
				ComponentName: componentName,
				FileNames:     []string{},
			}
		}

		fileName := projectFile.File.Path
		tmp[componentName].FileNames = append(
			tmp[componentName].FileNames,
			fileName,
		)
	}

	mapping := make([]models.CmdMappingOutGrouped, 0)
	for _, component := range spec.Components {
		componentName := component.Name.Value
		if grouped, exist := tmp[componentName]; exist {
			sort.Strings(grouped.FileNames)
			mapping = append(mapping, *grouped)
			continue
		}

		mapping = append(mapping, models.CmdMappingOutGrouped{
			ComponentName: componentName,
			FileNames:     []string{},
		})
	}

	emptyComponentID := componentName(nil)
	if _, hasNotAttached := tmp[emptyComponentID]; hasNotAttached {
		notAttachedFiles := tmp[emptyComponentID].FileNames

		if len(notAttachedFiles) > 0 {
			sort.Strings(notAttachedFiles)
			mapping = append(mapping, models.CmdMappingOutGrouped{
				ComponentName: emptyComponentID,
				FileNames:     notAttachedFiles,
			})
		}
	}

	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i].ComponentName < mapping[j].ComponentName
	})

	return mapping
}

func assembleMappingByFile(projectFiles []models.FileHold) []models.CmdMappingOutList {
	mapping := make([]models.CmdMappingOutList, 0)
	exist := make(map[string]struct{})

	for _, projectFile := range projectFiles {
		fileName := projectFile.File.Path

		if _, exist := exist[fileName]; exist {
			continue
		}

		mapping = append(mapping, models.CmdMappingOutList{
			FileName:      fileName,
			ComponentName: componentName(projectFile.ComponentID),
		})

		exist[fileName] = struct{}{}
	}

	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i].FileName < mapping[j].FileName
	})

	return mapping
}

func componentName(id *string) string {
	if id == nil {
		return "[not attached]"
	}

	return *id
}
