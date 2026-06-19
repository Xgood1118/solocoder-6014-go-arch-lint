package models

const (
	MappingSchemeGrouped MappingScheme = "grouped"
	MappingSchemeList    MappingScheme = "list"
)

var MappingSchemesValues = []string{
	MappingSchemeList,
	MappingSchemeGrouped,
}

type (
	MappingScheme = string

	CmdMappingIn struct {
		ProjectPath string
		ArchFile    string
		Scheme      MappingScheme
		FilterGlob  string
	}

	CmdMappingOut struct {
		ProjectDirectory string                 `json:"ProjectDirectory"`
		ModuleName       string                 `json:"ModuleName"`
		MappingGrouped   []CmdMappingOutGrouped `json:"MappingGrouped"`
		MappingList      []CmdMappingOutList    `json:"MappingList"`
		Scheme           MappingScheme          `json:"-"`
		FilterGlob       string                 `json:"FilterGlob,omitempty"`
	}

	CmdMappingOutGrouped struct {
		ComponentName string
		FileNames     []string
	}

	CmdMappingOutList struct {
		FileName      string
		ComponentName string
	}
)
