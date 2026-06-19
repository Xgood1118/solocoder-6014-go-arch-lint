package container

import (
	"github.com/spf13/cobra"

	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/operations/cache"
)

func (c *Container) commandCache() (*cobra.Command, runner) {
	cmd := &cobra.Command{
		Use:     "cache",
		Aliases: []string{"cc"},
		Short:   "generate imports cache for faster check runs",
		Long:    "scan project imports and store results in .go-arch-lint.cache file for subsequent check commands to reuse",
	}

	in := models.CmdCacheIn{
		ProjectPath: models.DefaultProjectPath,
		ArchFile:    models.DefaultArchFileName,
	}

	cmd.PersistentFlags().StringVar(&in.ProjectPath, "project-path", in.ProjectPath, "absolute path to project directory")
	cmd.PersistentFlags().StringVar(&in.ArchFile, "arch-file", in.ArchFile, "arch file path")

	return cmd, func(act *cobra.Command) (any, error) {
		return c.commandCacheOperation().Behave(act.Context(), in)
	}
}

func (c *Container) commandCacheOperation() *cache.Operation {
	return cache.NewOperation(
		c.provideProjectInfoAssembler(),
		c.provideSpecAssembler(),
		c.provideProjectFilesScanner(),
		c.provideCacheService(),
	)
}
