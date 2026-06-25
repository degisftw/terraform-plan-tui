// Package tfplan parses the JSON output of `terraform show -json <planfile>`
// into a navigable domain model: a block-level dependency graph plus
// per-attribute before/after diffs.
package tfplan

import (
	"strconv"
	"strings"
)

// Action is the high-level change a plan proposes for a resource instance.
type Action string

const (
	ActionNoOp    Action = "no-op"
	ActionCreate  Action = "create"
	ActionRead    Action = "read"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionReplace Action = "replace" // delete+create or create+delete
	ActionForget  Action = "forget"  // removed from state, kept in the real world
	ActionMixed   Action = "mixed"   // a multi-instance block with differing actions
)

// actionFromList collapses Terraform's actions array into a single Action.
func actionFromList(actions []string) Action {
	switch len(actions) {
	case 0:
		return ActionNoOp
	case 1:
		switch actions[0] {
		case "no-op":
			return ActionNoOp
		case "create":
			return ActionCreate
		case "read":
			return ActionRead
		case "update":
			return ActionUpdate
		case "delete":
			return ActionDelete
		case "forget":
			return ActionForget
		}
	case 2:
		// ["delete","create"] or ["create","delete"] => replace
		return ActionReplace
	}
	return ActionNoOp
}

// Symbol returns the single-character glyph Terraform-style tools use for an action.
func (a Action) Symbol() string {
	switch a {
	case ActionCreate:
		return "+"
	case ActionUpdate:
		return "~"
	case ActionDelete:
		return "-"
	case ActionReplace:
		return "±"
	case ActionRead:
		return "≤"
	case ActionForget:
		return "⨯"
	case ActionMixed:
		return "*"
	default:
		return "·"
	}
}

// Label is a short human word for the action.
func (a Action) Label() string {
	switch a {
	case ActionNoOp:
		return "no-op"
	default:
		return string(a)
	}
}

// Resource is a single planned instance (one entry in resource_changes).
type Resource struct {
	Block  string // address without instance index, e.g. random_pet.app
	Module string // module_address, "" for root
	Type   string
	Name   string
	Index  string // "", "0", or "\"key\""
	Action Action

	Before          map[string]any
	After           map[string]any
	AfterUnknown    map[string]any
	BeforeSensitive any
	AfterSensitive  any
	ReplacePaths    [][]any

	changed int             // memoized ChangedAttrs (-1 = unset)
	forces  map[string]bool // memoized replace-forcing attribute paths
}

// Node is a resource block: one or more instances that share a config block and
// therefore a position in the dependency graph.
type Node struct {
	Block     string // address without index
	Type      string
	Name      string
	Module    string
	Instances []*Resource

	DepsOut []string // block addresses this node depends on (upstream)
	DepsIn  []string // block addresses that depend on this node (downstream)

	Layer int // column: 0 = no dependencies, grows rightward
	Row   int // position within the layer

	action Action // cached aggregate action
}

// ActionAgg returns the aggregate action for the block: the shared action if all
// instances agree, otherwise ActionMixed.
func (n *Node) ActionAgg() Action {
	if n.action != "" {
		return n.action
	}
	if len(n.Instances) == 0 {
		n.action = ActionNoOp
		return n.action
	}
	first := n.Instances[0].Action
	for _, r := range n.Instances[1:] {
		if r.Action != first {
			n.action = ActionMixed
			return n.action
		}
	}
	n.action = first
	return n.action
}

// Label returns the "type.name" shown for a node.
func (n *Node) Label() string {
	if n.Module != "" {
		return n.Module + "." + n.Type + "." + n.Name
	}
	return n.Type + "." + n.Name
}

// Count is the number of instances under the block.
func (n *Node) Count() int { return len(n.Instances) }

// Summary holds plan-wide counts (instance level, matching terraform's footer).
type Summary struct {
	Add     int
	Change  int
	Destroy int
	Replace int // subset: instances that are replacements
	Read    int
	Forget  int
	NoOp    int
}

// Graph is the whole parsed plan.
type Graph struct {
	Nodes   []*Node
	ByBlock map[string]*Node
	Layers  [][]*Node // Layers[c] = column c, ordered top to bottom

	Summary          Summary
	FormatVersion    string
	TerraformVersion string
	PlanPath         string // source file, for display
}

// MaxLayer returns the index of the rightmost column (or -1 if empty).
func (g *Graph) MaxLayer() int { return len(g.Layers) - 1 }

// formatIndex renders a resource_changes "index" field (number or string key).
func formatIndex(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case float64:
		// integer index from count
		return strconv.FormatInt(int64(t), 10)
	case string:
		return strconv.Quote(t)
	default:
		return ""
	}
}

// blockAddress strips a trailing instance index ("[...]") from an address.
func blockAddress(addr string) string {
	if i := strings.LastIndexByte(addr, '['); i >= 0 && strings.HasSuffix(addr, "]") {
		return addr[:i]
	}
	return addr
}
