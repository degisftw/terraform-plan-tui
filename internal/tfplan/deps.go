package tfplan

import (
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// A resourceReference retains the resource instance traversal when present.
// Dynamic indexes remain literal traversal text, never an inferred pairing.
type resourceReference struct {
	block   string
	index   string
	indexed bool
}

// collectModuleDeps walks configuration resources and their expression references.
// prefix is the module address prefix, e.g. "module.foo." for child modules.
func collectModuleDeps(m rawConfigModule, prefix string, out map[string]map[resourceReference]bool) {
	for _, r := range m.Resources {
		block := prefix + r.Address
		set := out[block]
		if set == nil {
			set = map[resourceReference]bool{}
			out[block] = set
		}
		for _, expr := range r.Expressions {
			for _, ref := range extractReferences(expr) {
				if dep, ok := parseReference(ref, prefix); ok && dep.block != block {
					set[dep] = true
				}
			}
		}
		// depends_on is independent evidence, not an implied expression prefix.
		for _, ref := range r.DependsOn {
			if dep, ok := parseReference(ref, prefix); ok && dep.block != block {
				set[dep] = true
			}
		}
	}
	for name, call := range m.ModuleCalls {
		collectModuleDeps(call.Module, prefix+"module."+name+".", out)
	}
}

// extractReferences recursively pulls references from expression values. Terraform
// includes shorter traversal prefixes alongside each full reference. Remove those
// only within their own references array, so independent ambiguous expressions or
// nested blocks cannot be erased by an indexed reference elsewhere.
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
				dynamicIndex := false
				for _, e := range r {
					if ref, ok := e.(string); ok {
						root, _, _ := strings.Cut(ref, ".")
						switch root {
						case "var", "local", "each", "count", "module":
							dynamicIndex = true
						}
					}
				}
				for _, e := range r {
					ref, ok := e.(string)
					if !ok {
						continue
					}
					implied := false
					for _, other := range r {
						longer, ok := other.(string)
						if ok && len(longer) > len(ref) && strings.HasPrefix(longer, ref) &&
							(longer[len(ref)] == '.' || longer[len(ref)] == '[') {
							// These may also encode a dynamic lookup whose index was
							// omitted from the JSON traversal; keep its block scope.
							if dynamicIndex && longer[len(ref)] == '[' && literalReferenceIndex(longer[len(ref):]) {
								continue
							}
							implied = true
							break
						}
					}
					if !implied {
						refs = append(refs, ref)
					}
				}
			}
			for k, child := range t {
				if k != "references" {
					walk(child)
				}
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

// parseReference preserves the first resource index, including literal string
// keys with dots or brackets. Dynamic indexes cannot match a planned literal index.
// Cross-module outputs remain unsupported, matching the existing block graph's
// module-local reference behavior.
func parseReference(ref, prefix string) (resourceReference, bool) {
	resourceType, rest, ok := strings.Cut(ref, ".")
	if !ok {
		return resourceReference{}, false
	}
	switch resourceType {
	case "var", "local", "each", "count", "path", "terraform", "self", "module":
		return resourceReference{}, false
	case "data":
		dataType, remainder, ok := strings.Cut(rest, ".")
		if !ok {
			return resourceReference{}, false
		}
		resourceType = "data." + dataType
		rest = remainder
	}
	nameEnd := strings.IndexAny(rest, ".[")
	if nameEnd < 0 {
		nameEnd = len(rest)
	}
	if nameEnd == 0 {
		return resourceReference{}, false
	}
	dep := resourceReference{block: prefix + resourceType + "." + rest[:nameEnd]}
	if nameEnd < len(rest) && rest[nameEnd] == '[' {
		index := rest[nameEnd:]
		dep.indexed = true
		if end := referenceIndexEnd(index); end >= 0 {
			dep.index = index[1 : end-1]
		} else {
			dep.index = index
		}
	}
	return dep, true
}

// referenceIndexEnd returns the byte after a complete bracket traversal, ignoring
// brackets inside quoted keys and supporting nested dynamic index expressions.
func referenceIndexEnd(s string) int {
	depth := 0
	quoted, escaped := false, false
	for i := range len(s) {
		c := s[i]
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		switch c {
		case '"':
			quoted = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func literalReferenceIndex(s string) bool {
	end := referenceIndexEnd(s)
	if end < 0 {
		return false
	}
	index := s[1 : end-1]
	if strings.HasPrefix(index, "\"") {
		_, err := strconv.Unquote(index)
		return err == nil
	}
	_, err := strconv.ParseUint(index, 10, 64)
	return err == nil
}

// applyEdges retains the legacy block graph and adds confirmed instance addresses
// plus an explicit marker for any relationships that remain block-scoped.
func (g *Graph) applyEdges(deps map[string]map[resourceReference]bool) {
	for block, refs := range deps {
		node := g.ByBlock[block]
		if node == nil {
			continue
		}
		grouped := map[string]*InstanceDependency{}
		for ref := range refs {
			upstream := g.ByBlock[ref.block]
			if upstream == nil {
				continue
			}
			dep := grouped[ref.block]
			if dep == nil {
				dep = &InstanceDependency{Block: ref.block}
				grouped[ref.block] = dep
			}
			address := ""
			if !ref.indexed {
				if len(upstream.Instances) == 1 {
					address = upstream.Instances[0].Address()
				}
			} else {
				for _, instance := range upstream.Instances {
					if instance.Index != "" && instance.Index == ref.index {
						address = instance.Address()
						break
					}
				}
			}
			if address == "" {
				dep.Unresolved = true
			} else if !slices.Contains(dep.Instances, address) {
				dep.Instances = append(dep.Instances, address)
			}
		}
		for _, dep := range grouped {
			sort.Strings(dep.Instances)
			node.InstanceDeps = append(node.InstanceDeps, *dep)
			node.DepsOut = append(node.DepsOut, dep.Block)
			g.ByBlock[dep.Block].DepsIn = append(g.ByBlock[dep.Block].DepsIn, block)
		}
		sort.Slice(node.InstanceDeps, func(i, j int) bool {
			return node.InstanceDeps[i].Block < node.InstanceDeps[j].Block
		})
	}
	for _, n := range g.Nodes {
		sort.Strings(n.DepsOut)
		sort.Strings(n.DepsIn)
	}
}
