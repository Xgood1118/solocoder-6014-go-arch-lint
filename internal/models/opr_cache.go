package models

const (
	CacheSchemaVersion       = 1
	CacheFileName            = ".go-arch-lint.cache"
)

type (
	CmdCacheIn struct {
		ProjectPath string
		ArchFile    string
	}

	CmdCacheOut struct {
		ProjectDirectory string `json:"ProjectDirectory"`
		ModuleName       string `json:"ModuleName"`
		CacheFile        string `json:"CacheFile"`
		FilesCached      int    `json:"FilesCached"`
	}

	CacheFile struct {
		SchemaVersion int                    `json:"SchemaVersion"`
		ConfigHash    string                 `json:"ConfigHash"`
		ModuleName    string                 `json:"ModuleName"`
		ProjectFiles  []CacheProjectFile     `json:"ProjectFiles"`
	}

	CacheProjectFile struct {
		Path    string               `json:"Path"`
		Imports []CacheResolvedImport `json:"Imports"`
	}

	CacheResolvedImport struct {
		Name       string `json:"Name"`
		ImportType uint8  `json:"ImportType"`
	}
)
