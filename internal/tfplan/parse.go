package tfplan

import (
	"encoding/json"
	"fmt"
)

// Parse converts the JSON output of `terraform show -json <planfile>` into a Graph.
// It runs a three-step pipeline: group instances into block-level nodes, wire the
// dependency edges from the configuration, then assign and order the graph layers.
func Parse(data []byte) (*Graph, error) {
	var raw rawPlan
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse plan json: %w", err)
	}
	if raw.FormatVersion == "" && len(raw.ResourceChanges) == 0 {
		return nil, fmt.Errorf("this does not look like terraform plan JSON (no format_version or resource_changes); did you run `terraform show -json <planfile>`?")
	}

	g := &Graph{
		ByBlock:          map[string]*Node{},
		FormatVersion:    raw.FormatVersion,
		TerraformVersion: raw.TerraformVersion,
	}

	// 1. Instances grouped into block-level nodes.
	for i := range raw.ResourceChanges {
		rc := &raw.ResourceChanges[i]
		res := buildResource(rc)
		g.tallySummary(res)

		node := g.ByBlock[res.Block]
		if node == nil {
			node = &Node{
				Block:  res.Block,
				Type:   res.Type,
				Name:   res.Name,
				Module: res.Module,
			}
			g.ByBlock[res.Block] = node
			g.Nodes = append(g.Nodes, node)
		}
		node.Instances = append(node.Instances, res)
	}

	// 2. Dependency edges from the configuration's expression references.
	deps := map[string]map[resourceReference]bool{}
	collectModuleDeps(raw.Configuration.RootModule, "", deps)
	g.applyEdges(deps)

	// 3. Layering + ordering.
	g.assignLayers()
	g.buildLayers()

	return g, nil
}

func buildResource(rc *rawResourceChange) *Resource {
	res := &Resource{
		Block:   blockAddress(rc.Address),
		Module:  rc.ModuleAddress,
		Type:    rc.Type,
		Name:    rc.Name,
		Index:   formatIndex(rc.Index),
		Action:  actionFromList(rc.Change.Actions),
		changed: -1,
	}
	res.Before = decodeObject(rc.Change.Before)
	res.After = decodeObject(rc.Change.After)
	res.AfterUnknown = decodeObject(rc.Change.AfterUnknown)
	res.BeforeSensitive = decodeAny(rc.Change.BeforeSensitive)
	res.AfterSensitive = decodeAny(rc.Change.AfterSensitive)
	if rp, ok := decodeAny(rc.Change.ReplacePaths).([]any); ok {
		for _, p := range rp {
			if steps, ok := p.([]any); ok {
				res.ReplacePaths = append(res.ReplacePaths, steps)
			}
		}
	}
	return res
}

// tallySummary adds one resource instance to the plan-wide counts.
func (g *Graph) tallySummary(r *Resource) {
	switch r.Action {
	case ActionCreate:
		g.Summary.Add++
	case ActionUpdate:
		g.Summary.Change++
	case ActionDelete:
		g.Summary.Destroy++
	case ActionReplace:
		g.Summary.Add++
		g.Summary.Destroy++
		g.Summary.Replace++
	case ActionRead:
		g.Summary.Read++
	case ActionForget:
		g.Summary.Forget++
	default:
		g.Summary.NoOp++
	}
}
