package tfplan

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// DiffKind classifies what happened to a single attribute (or nested leaf).
type DiffKind int

const (
	KindNoChange DiffKind = iota
	KindAdd
	KindRemove
	KindUpdate
	KindUnknown // known after apply
)

// DiffLine is one rendered attribute row in the expanded view.
type DiffLine struct {
	Indent    int    // nesting depth
	Key       string // attribute name (or sub-key)
	Kind      DiffKind
	Before    string // formatted, "" when not applicable
	After     string // formatted, "" when not applicable
	Sensitive bool
	IsParent  bool // a map header whose children follow
	Forces    bool // this attribute forces resource replacement
}

const unknownText = "(known after apply)"
const sensitiveText = "(sensitive value)"
const maxDiffDepth = 8

// Diff computes the ordered attribute diff for one resource instance.
// When showUnchanged is false, unchanged leaves are omitted.
func (r *Resource) Diff(showUnchanged bool) []DiffLine {
	var out []DiffLine
	dc := diffCtx{showUnchanged: showUnchanged, forces: r.forcesSet()}
	dc.diffMap(r.Before, r.After, r.AfterUnknown, r.BeforeSensitive, r.AfterSensitive, 0, "", &out)
	return out
}

// ChangedAttrs returns how many top-level attributes changed
// (add/remove/update/unknown). Result is memoized; before/after never change.
func (r *Resource) ChangedAttrs() int {
	if r.changed >= 0 {
		return r.changed
	}
	n := 0
	for _, l := range r.Diff(false) {
		if l.Indent == 0 && !l.IsParent && l.Kind != KindNoChange {
			n++
		}
	}
	r.changed = n
	return n
}

// diffCtx carries per-diff settings so we don't thread many params.
type diffCtx struct {
	showUnchanged bool
	forces        map[string]bool // attribute path -> forces replacement
}

func (dc diffCtx) diffMap(before, after map[string]any, unknown, beforeSens, afterSens any, indent int, path string, out *[]DiffLine) {
	if indent > maxDiffDepth {
		return
	}
	for _, key := range diffKeys(before, after, unknown) {
		bv, bok := before[key]
		av, aok := after[key]
		uk := childOf(unknown, key)
		// Sensitivity markers may be a bool, a nested map, OR an array (per-element
		// sensitivity for lists/sets). Redact if any descendant is sensitive.
		bsen := sensitive(childOf(beforeSens, key))
		asen := sensitive(childOf(afterSens, key))
		childPath := key
		if path != "" {
			childPath = path + "." + key
		}
		forces := dc.forces[childPath]

		// Recurse into nested maps when at least one side is a map.
		bm, bIsMap := bv.(map[string]any)
		am, aIsMap := av.(map[string]any)
		if (bIsMap || aIsMap) && indent < maxDiffDepth {
			before2 := bm
			after2 := am
			// Only descend if there is meaningful nested content.
			if len(before2) > 0 || len(after2) > 0 || isMapLike(uk) {
				kind := nestedKind(bok, aok, bm, am)
				if kind != KindNoChange || dc.showUnchanged {
					*out = append(*out, DiffLine{Indent: indent, Key: key, Kind: kind, IsParent: true, Forces: forces})
					dc.diffMap(before2, after2, uk, childOf(beforeSens, key), childOf(afterSens, key), indent+1, childPath, out)
				}
				continue
			}
		}

		line := DiffLine{Indent: indent, Key: key, Sensitive: bsen || asen, Forces: forces}
		switch {
		case truthy(uk):
			line.Kind = KindUnknown
			if bok {
				line.Before = leaf(bv, bsen)
			}
			line.After = unknownText
		case bok && aok:
			if reflect.DeepEqual(bv, av) {
				if !dc.showUnchanged {
					continue
				}
				line.Kind = KindNoChange
				line.After = leaf(av, asen)
			} else {
				line.Kind = KindUpdate
				line.Before = leaf(bv, bsen)
				line.After = leaf(av, asen)
			}
		case aok && !bok:
			line.Kind = KindAdd
			line.After = leaf(av, asen)
		case bok && !aok:
			line.Kind = KindRemove
			line.Before = leaf(bv, bsen)
		default:
			continue
		}
		*out = append(*out, line)
	}
}

// nestedKind classifies a map-valued attribute as a whole.
func nestedKind(bok, aok bool, bm, am map[string]any) DiffKind {
	switch {
	case !bok && aok, len(bm) == 0 && len(am) > 0:
		if !bok {
			return KindAdd
		}
	case bok && !aok, len(bm) > 0 && len(am) == 0:
		if !aok {
			return KindRemove
		}
	}
	if reflect.DeepEqual(bm, am) {
		return KindNoChange
	}
	return KindUpdate
}

// diffKeys is the sorted union of attribute names across the before map, the
// after map, and any keys present only in the unknown (known-after-apply) tree.
func diffKeys(before, after map[string]any, unknown any) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(m map[string]any) {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	add(before)
	add(after)
	if um, ok := unknown.(map[string]any); ok {
		add(um)
	}
	sort.Strings(keys)
	return keys
}

// childOf extracts the sub-node for key from an unknown/sensitive tree.
// A bool true means "the whole subtree applies", so it propagates downward.
func childOf(tree any, key string) any {
	switch t := tree.(type) {
	case bool:
		if t {
			return true
		}
		return nil
	case map[string]any:
		return t[key]
	default:
		return nil
	}
}

func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

// sensitive reports whether any descendant of a before_sensitive/after_sensitive
// sub-tree is marked sensitive. The marker may be a bool, a nested object, or an
// array (per-element sensitivity for lists/sets) — all are handled so that a
// sensitive value nested in a collection is never rendered in plaintext.
func sensitive(tree any) bool {
	switch t := tree.(type) {
	case bool:
		return t
	case map[string]any:
		for _, v := range t {
			if sensitive(v) {
				return true
			}
		}
	case []any:
		for _, v := range t {
			if sensitive(v) {
				return true
			}
		}
	}
	return false
}

// forcesSet returns the set of attribute paths (dotted) that force replacement,
// derived from the plan's replace_paths. Memoized.
func (r *Resource) forcesSet() map[string]bool {
	if r.forces != nil {
		return r.forces
	}
	s := map[string]bool{}
	for _, p := range r.ReplacePaths {
		parts := make([]string, 0, len(p))
		for _, step := range p {
			switch v := step.(type) {
			case string:
				parts = append(parts, v)
			case float64:
				parts = append(parts, strconv.FormatInt(int64(v), 10))
			case json.Number:
				parts = append(parts, v.String())
			}
		}
		if len(parts) > 0 {
			s[strings.Join(parts, ".")] = true
		}
	}
	r.forces = s
	return s
}

func isMapLike(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		return true
	case bool:
		return t
	}
	return false
}

// leaf formats a scalar/collection value for display.
func leaf(v any, sensitive bool) string {
	if sensitive {
		return sensitiveText
	}
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case string:
		return strconv.Quote(t)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "?"
		}
		return string(b)
	}
}
