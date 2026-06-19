package graph

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"

	"github.com/fe3dback/go-arch-lint/internal/models"
	"github.com/fe3dback/go-arch-lint/internal/models/arch"
)

type Operation struct {
	specAssembler        specAssembler
	projectInfoAssembler projectInfoAssembler
}

func NewOperation(
	specAssembler specAssembler,
	projectInfoAssembler projectInfoAssembler,
) *Operation {
	return &Operation{
		specAssembler:        specAssembler,
		projectInfoAssembler: projectInfoAssembler,
	}
}

func (o *Operation) Behave(ctx context.Context, in models.CmdGraphIn) (models.CmdGraphOut, error) {
	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to assemble project info: %w", err)
	}

	spec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to assemble spec: %w", err)
	}

	if in.Type == models.GraphTypeDiff {
		return o.behaveDiff(ctx, in, spec)
	}

	return o.behaveNormal(ctx, in, spec)
}

func (o *Operation) behaveNormal(ctx context.Context, in models.CmdGraphIn, spec arch.Spec) (models.CmdGraphOut, error) {
	graphCode, err := o.buildGraph(spec, in)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed build graph: %w", err)
	}

	svg, err := o.compileGraph(ctx, graphCode)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to compile graph: %w", err)
	}

	outFile, err := filepath.Abs(in.OutFile)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed get abs path from '%s': %w", in.OutFile, err)
	}

	if o.isFileShouldBeWritten(in) {
		err = os.WriteFile(outFile, svg, 0o600)
		if err != nil {
			return models.CmdGraphOut{}, fmt.Errorf("failed write graph into '%s' file: %w", in.OutFile, err)
		}
	}

	return models.CmdGraphOut{
		ProjectDirectory: spec.RootDirectory.Value,
		ModuleName:       spec.ModuleName.Value,
		OutFile:          outFile,
		D2Definitions:    string(graphCode),
		ExportD2:         in.ExportD2,
	}, nil
}

func (o *Operation) behaveDiff(ctx context.Context, in models.CmdGraphIn, spec arch.Spec) (models.CmdGraphOut, error) {
	if in.DiffFrom == "" || in.DiffTo == "" {
		return models.CmdGraphOut{}, fmt.Errorf("diff mode requires --diff-from and --diff-to commit refs")
	}

	fromEdges, err := o.edgesAtCommit(in, in.DiffFrom)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to resolve edges at '%s': %w", in.DiffFrom, err)
	}

	toEdges, err := o.edgesAtCommit(in, in.DiffTo)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to resolve edges at '%s': %w", in.DiffTo, err)
	}

	added, removed := diffEdges(fromEdges, toEdges)

	graphCode, err := o.buildDiffGraph(added, removed)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed build diff graph: %w", err)
	}

	svg, err := o.compileGraph(ctx, graphCode)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed to compile diff graph: %w", err)
	}

	outFile, err := filepath.Abs(in.OutFile)
	if err != nil {
		return models.CmdGraphOut{}, fmt.Errorf("failed get abs path from '%s': %w", in.OutFile, err)
	}

	if o.isFileShouldBeWritten(in) {
		err = os.WriteFile(outFile, svg, 0o600)
		if err != nil {
			return models.CmdGraphOut{}, fmt.Errorf("failed write diff graph into '%s' file: %w", in.OutFile, err)
		}
	}

	return models.CmdGraphOut{
		ProjectDirectory: spec.RootDirectory.Value,
		ModuleName:       spec.ModuleName.Value,
		OutFile:          outFile,
		D2Definitions:    string(graphCode),
		ExportD2:         in.ExportD2,
		IsDiff:           true,
		DiffAddedEdges:   added,
		DiffRemovedEdges: removed,
	}, nil
}

type edgeKey struct {
	From string
	To   string
}

func (o *Operation) edgesAtCommit(in models.CmdGraphIn, commitRef string) (map[edgeKey]struct{}, error) {
	resolvedRef, err := o.resolveCommitRef(commitRef, in.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve commit ref '%s': %w", commitRef, err)
	}

	stashErr := o.gitStash(in.ProjectPath)
	if stashErr == nil {
		defer o.gitStashPop(in.ProjectPath)
	}

	checkoutErr := o.gitCheckout(resolvedRef, in.ProjectPath)
	if checkoutErr != nil {
		return nil, fmt.Errorf("failed to checkout '%s': %w", resolvedRef, checkoutErr)
	}
	defer o.gitCheckout("-", in.ProjectPath)

	projectInfo, err := o.projectInfoAssembler.ProjectInfo(in.ProjectPath, in.ArchFile)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble project info at '%s': %w", resolvedRef, err)
	}

	commitSpec, err := o.specAssembler.Assemble(projectInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to assemble spec at '%s': %w", resolvedRef, err)
	}

	edges := o.collectEdges(commitSpec, in)
	return edges, nil
}

func (o *Operation) collectEdges(spec arch.Spec, opts models.CmdGraphIn) map[edgeKey]struct{} {
	whiteList, err := o.populateGraphWhitelist(spec, opts)
	if err != nil {
		return nil
	}

	edges := make(map[edgeKey]struct{})
	for _, cmp := range spec.Components {
		if _, visible := whiteList[cmp.Name.Value]; !visible {
			continue
		}

		for _, dep := range cmp.MayDependOn {
			if _, visible := whiteList[dep.Value]; !visible {
				continue
			}

			edges[edgeKey{From: cmp.Name.Value, To: dep.Value}] = struct{}{}
		}
	}

	return edges
}

func diffEdges(from, to map[edgeKey]struct{}) (added, removed []models.GraphDiffEdge) {
	added = make([]models.GraphDiffEdge, 0)
	removed = make([]models.GraphDiffEdge, 0)

	for k := range to {
		if _, exists := from[k]; !exists {
			added = append(added, models.GraphDiffEdge{From: k.From, To: k.To})
		}
	}

	for k := range from {
		if _, exists := to[k]; !exists {
			removed = append(removed, models.GraphDiffEdge{From: k.From, To: k.To})
		}
	}

	sort.Slice(added, func(i, j int) bool {
		if added[i].From != added[j].From {
			return added[i].From < added[j].From
		}
		return added[i].To < added[j].To
	})

	sort.Slice(removed, func(i, j int) bool {
		if removed[i].From != removed[j].From {
			return removed[i].From < removed[j].From
		}
		return removed[i].To < removed[j].To
	})

	return added, removed
}

func (o *Operation) resolveCommitRef(ref string, projectPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse '%s' failed: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (o *Operation) gitCheckout(ref string, projectPath string) error {
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = projectPath
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (o *Operation) gitStash(projectPath string) error {
	cmd := exec.Command("git", "stash", "--include-untracked")
	cmd.Dir = projectPath
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return nil
}

func (o *Operation) gitStashPop(projectPath string) error {
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = projectPath
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return nil
}

func (o *Operation) buildDiffGraph(added, removed []models.GraphDiffEdge) ([]byte, error) {
	linesBuff := make([]string, 0, 256)

	for _, edge := range added {
		linesBuff = append(linesBuff, fmt.Sprintf("%s -> %s {\n  style.stroke: \"#22AA44\"\n  style.font-size: 12\n  label: \"added\"\n}\n", edge.From, edge.To))
	}

	for _, edge := range removed {
		linesBuff = append(linesBuff, fmt.Sprintf("%s -> %s {\n  style.stroke: \"#CC3333\"\n  style.stroke-dash: 5\n  style.font-size: 12\n  label: \"removed\"\n}\n", edge.From, edge.To))
	}

	var buff bytes.Buffer
	sort.Strings(linesBuff)

	for _, line := range linesBuff {
		buff.WriteString(strings.ReplaceAll(line, "\t", ""))
	}

	return buff.Bytes(), nil
}

func (o *Operation) isFileShouldBeWritten(in models.CmdGraphIn) bool {
	if in.OutputType == models.OutputTypeJSON {
		return false
	}

	if in.ExportD2 {
		return false
	}

	return true
}

func (o *Operation) buildGraph(spec arch.Spec, opts models.CmdGraphIn) ([]byte, error) {
	whiteList, err := o.populateGraphWhitelist(spec, opts)
	if err != nil {
		return nil, err
	}

	flow := o.componentsFlowArrow(opts)

	linesBuff := make([]string, 0, 256)

	for _, cmp := range spec.Components {
		if _, visible := whiteList[cmp.Name.Value]; !visible {
			continue
		}

		for _, dep := range cmp.MayDependOn {
			if _, visible := whiteList[dep.Value]; !visible {
				continue
			}

			linesBuff = append(linesBuff, fmt.Sprintf("%s %s %s\n", cmp.Name.Value, flow, dep.Value))
		}

		if opts.IncludeVendors {
			for _, vnd := range cmp.CanUse {
				vars := map[string]string{
					"vnd": vnd.Value,
					"cmp": cmp.Name.Value,
				}

				tpl := `
				{{vnd}}.style.font-size: 12
				{{vnd}}.style.stroke: "#77AA44"
				{{cmp}} <- {{vnd}} {
				  style.stroke: "#77AA44"
				  source-arrowhead: {
				    shape: diamond
				    style.filled: false
				  }
				}
				`

				for name, value := range vars {
					tpl = strings.ReplaceAll(tpl, fmt.Sprintf("{{%s}}", name), value)
				}
				linesBuff = append(linesBuff, tpl)
			}
		}
	}

	var buff bytes.Buffer
	sort.Strings(linesBuff)

	for _, line := range linesBuff {
		buff.WriteString(strings.ReplaceAll(line, "\t", ""))
	}

	return buff.Bytes(), nil
}

func (o *Operation) componentsFlowArrow(opts models.CmdGraphIn) string {
	if opts.Type == models.GraphTypeFlow {
		return "->"
	}

	if opts.Type == models.GraphTypeDI {
		return "<-"
	}

	return "--"
}

func (o *Operation) populateGraphWhitelist(spec arch.Spec, opts models.CmdGraphIn) (map[string]struct{}, error) {
	if opts.Focus == "" {
		return o.populateGraphWhitelistAll(spec)
	}

	return o.populateGraphWhitelistFocused(spec, opts.Focus)
}

func (o *Operation) populateGraphWhitelistAll(spec arch.Spec) (map[string]struct{}, error) {
	whiteList := make(map[string]struct{}, len(spec.Components))

	for _, cmp := range spec.Components {
		whiteList[cmp.Name.Value] = struct{}{}
	}

	return whiteList, nil
}

func (o *Operation) populateGraphWhitelistFocused(spec arch.Spec, focusCmpName string) (map[string]struct{}, error) {
	cmpMap := make(map[string]arch.Component)
	rootExist := false

	for _, cmp := range spec.Components {
		cmpMap[cmp.Name.Value] = cmp

		if focusCmpName == cmp.Name.Value {
			rootExist = true
		}
	}

	if !rootExist {
		return nil, fmt.Errorf("focused cmp %s is not defined", focusCmpName)
	}

	whiteList := make(map[string]struct{}, len(spec.Components))
	resolved := make(map[string]struct{}, 64)
	resolveList := make([]string, 0, 64)
	resolveList = append(resolveList, focusCmpName)

	for len(resolveList) > 0 {
		cmp := cmpMap[resolveList[0]]
		resolveList = resolveList[1:]

		if _, alreadyResolved := resolved[cmp.Name.Value]; alreadyResolved {
			continue
		}

		whiteList[cmp.Name.Value] = struct{}{}

		for _, dep := range cmp.MayDependOn {
			whiteList[dep.Value] = struct{}{}
			resolveList = append(resolveList, dep.Value)
		}

		resolved[cmp.Name.Value] = struct{}{}
	}

	return whiteList, nil
}

func (o *Operation) compileGraph(ctx context.Context, graphCode []byte) ([]byte, error) {
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("failed create ruler: %w", err)
	}

	diagram, _, err := d2lib.Compile(ctx, string(graphCode), &d2lib.CompileOptions{
		Layout: func(ctx context.Context, g *d2graph.Graph) error {
			return d2dagrelayout.Layout(ctx, g, nil)
		},
		Ruler: ruler,
	})
	if err != nil {
		return nil, fmt.Errorf("failed compile d2 graph: %w", err)
	}

	out, err := d2svg.Render(diagram, &d2svg.RenderOpts{
		Pad:     d2svg.DEFAULT_PADDING,
		Sketch:  true,
		ThemeID: d2themescatalog.NeutralDefault.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("svg render failed: %w", err)
	}

	return out, nil
}
