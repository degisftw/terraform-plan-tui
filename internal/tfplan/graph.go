package tfplan

import "strings"

// At returns the node at (col, row), or nil if out of range.
func (g *Graph) At(col, row int) *Node {
	if col < 0 || col >= len(g.Layers) {
		return nil
	}
	column := g.Layers[col]
	if row < 0 || row >= len(column) {
		return nil
	}
	return column[row]
}

// ColLen returns the number of nodes in a column.
func (g *Graph) ColLen(col int) int {
	if col < 0 || col >= len(g.Layers) {
		return 0
	}
	return len(g.Layers[col])
}

// TotalNodes is the number of resource blocks.
func (g *Graph) TotalNodes() int { return len(g.Nodes) }

// TotalInstances is the number of planned resource instances.
func (g *Graph) TotalInstances() int {
	n := 0
	for _, node := range g.Nodes {
		n += len(node.Instances)
	}
	return n
}

// Locate returns the column/row of a node by block address.
func (g *Graph) Locate(block string) (col, row int, ok bool) {
	n := g.ByBlock[block]
	if n == nil {
		return 0, 0, false
	}
	return n.Layer, n.Row, true
}

// Search finds the next node at or after (fromCol,fromRow) whose address
// contains query (case-insensitive), scanning column-major. Returns ok=false
// if none match.
func (g *Graph) Search(query string, fromCol, fromRow int) (col, row int, ok bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0, 0, false
	}
	// Build a flat, column-major ordering and search forward with wraparound.
	type pos struct{ c, r int }
	var order []pos
	for c := range g.Layers {
		for r := range g.Layers[c] {
			order = append(order, pos{c, r})
		}
	}
	start := 0
	for i, p := range order {
		if p.c == fromCol && p.r == fromRow {
			start = i + 1
			break
		}
	}
	for i := 0; i < len(order); i++ {
		p := order[(start+i)%len(order)]
		if strings.Contains(strings.ToLower(g.Layers[p.c][p.r].Block), query) {
			return p.c, p.r, true
		}
	}
	return 0, 0, false
}

// setOf builds a lookup set from a slice of block addresses.
func setOf(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it] = true
	}
	return s
}

// DepsOutSet returns the upstream neighbor blocks of n.
func (n *Node) DepsOutSet() map[string]bool { return setOf(n.DepsOut) }

// DepsInSet returns the downstream neighbor blocks of n.
func (n *Node) DepsInSet() map[string]bool { return setOf(n.DepsIn) }
