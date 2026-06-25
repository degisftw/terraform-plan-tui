package tfplan

import (
	"encoding/json"
	"sort"
	"strings"
)

// This file derives the block-level dependency edges from the plan's
// configuration: which resource blocks reference which, and wiring those
// references onto the graph as DepsOut/DepsIn.

// collectModuleDeps walks a configuration module (recursing into module calls),
// recording for each resource block the set of resource blocks it references.
// prefix is the module address prefix, e.g. "module.foo." for child modules.
func collectModuleDeps(m rawConfigModule, prefix string, out map[string]map[string]bool) {
	for _, r := range m.Resources {
		block := prefix + r.Address
		set := out[block]
		if set == nil {
			set = map[string]bool{}
			out[block] = set
		}
		for _, expr := range r.Expressions {
			for _, ref := range extractReferences(expr) {
				if addr, ok := refToAddress(ref, prefix); ok && addr != block {
					set[addr] = true
				}
			}
		}
		for _, ref := range r.DependsOn {
			if addr, ok := refToAddress(ref, prefix); ok && addr != block {
				set[addr] = true
			}
		}
	}
	for name, call := range m.ModuleCalls {
		collectModuleDeps(call.Module, prefix+"module."+name+".", out)
	}
}

// extractReferences recursively pulls every string under any "references" array
// in an expression value (which may be a scalar, object, or nested block list).
func extractReferences(raw json.RawMessage) []string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	var refs []string
	var walk func(any)
	walk = func(node any) {
		switch t := node.(type) {
		case map[string]any:
			if r, ok := t["references"].([]any); ok {
				for _, e := range r {
					if s, ok := e.(string); ok {
						refs = append(refs, s)
					}
				}
			}
			for k, child := range t {
				if k == "references" {
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range t {
				walk(child)
			}
		}
	}
	walk(v)
	return refs
}

// refToAddress maps a Terraform reference (e.g. "random_pet.app.id") to a
// managed/data resource block address, honoring the current module prefix.
// It returns ok=false for references that are not resources (var/local/etc).
func refToAddress(ref, prefix string) (string, bool) {
	parts := strings.Split(ref, ".")
	if len(parts) < 2 {
		return "", false
	}
	switch parts[0] {
	case "var", "local", "each", "count", "path", "terraform", "self":
		return "", false
	case "module":
		// Cross-module reference; intra-module edges are handled per-module.
		return "", false
	case "data":
		if len(parts) < 3 {
			return "", false
		}
		return prefix + "data." + stripIndex(parts[1]) + "." + stripIndex(parts[2]), true
	default:
		return prefix + stripIndex(parts[0]) + "." + stripIndex(parts[1]), true
	}
}

func stripIndex(s string) string {
	if i := strings.IndexByte(s, '['); i >= 0 {
		return s[:i]
	}
	return s
}

// applyEdges wires DepsOut/DepsIn on nodes from the block->upstream map, keeping
// only edges whose endpoints both exist in the plan.
func (g *Graph) applyEdges(deps map[string]map[string]bool) {
	for block, set := range deps {
		node := g.ByBlock[block]
		if node == nil {
			continue
		}
		for up := range set {
			if g.ByBlock[up] == nil {
				continue
			}
			node.DepsOut = append(node.DepsOut, up)
			g.ByBlock[up].DepsIn = append(g.ByBlock[up].DepsIn, block)
		}
	}
	for _, n := range g.Nodes {
		sort.Strings(n.DepsOut)
		sort.Strings(n.DepsIn)
	}
}
