package models

const (
	GraphTypeFlow GraphType = "flow"
	GraphTypeDI   GraphType = "di"
	GraphTypeDiff GraphType = "diff"
)

var GraphTypesValues = []string{
	GraphTypeFlow,
	GraphTypeDI,
	GraphTypeDiff,
}

type (
	GraphType = string

	CmdGraphIn struct {
		ProjectPath    string
		ArchFile       string
		Type           GraphType
		OutFile        string
		Focus          string
		IncludeVendors bool
		ExportD2       bool
		OutputType     OutputType
		DiffFrom       string
		DiffTo         string
	}

	CmdGraphOut struct {
		ProjectDirectory string `json:"ProjectDirectory"`
		ModuleName       string `json:"ModuleName"`
		OutFile          string `json:"OutFile"`
		D2Definitions    string `json:"D2Definitions"`
		ExportD2         bool   `json:"-"`
		IsDiff           bool   `json:"IsDiff"`
		DiffAddedEdges   []GraphDiffEdge `json:"DiffAddedEdges,omitempty"`
		DiffRemovedEdges []GraphDiffEdge `json:"DiffRemovedEdges,omitempty"`
	}

	GraphDiffEdge struct {
		From string `json:"From"`
		To   string `json:"To"`
	}
)
