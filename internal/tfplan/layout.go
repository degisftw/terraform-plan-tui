package tfplan

import "sort"

// This file turns the wired dependency graph into a columnar layout: each node
// gets a layer (longest path from a root) and a deterministic row within it.

// assignLayers gives each node a layer = longest dependency path from a root,
// so resources with no dependencies sit in column 0 and dependents grow rightward.
func (g *Graph) assignLayers() {
	memo := map[string]int{}
	var layer func(block string, stack map[string]bool) int
	layer = func(block string, stack map[string]bool) int {
		if v, ok := memo[block]; ok {
			return v
		}
		if stack[block] {
			return 0 // cycle guard (shouldn't happen in a valid plan)
		}
		stack[block] = true
		max := -1
		for _, up := range g.ByBlock[block].DepsOut {
			if g.ByBlock[up] == nil {
				continue
			}
			if l := layer(up, stack); l > max {
				max = l
			}
		}
		delete(stack, block)
		res := max + 1
		memo[block] = res
		return res
	}
	for _, n := range g.Nodes {
		n.Layer = layer(n.Block, map[string]bool{})
	}
}

// buildLayers groups nodes into ordered columns.
func (g *Graph) buildLayers() {
	if len(g.Nodes) == 0 {
		g.Layers = nil
		return
	}
	maxLayer := 0
	for _, n := range g.Nodes {
		if n.Layer > maxLayer {
			maxLayer = n.Layer
		}
	}
	g.Layers = make([][]*Node, maxLayer+1)
	for _, n := range g.Nodes {
		g.Layers[n.Layer] = append(g.Layers[n.Layer], n)
	}
	for c := range g.Layers {
		sortNodesByAddress(g.Layers[c])
		for r, n := range g.Layers[c] {
			n.Row = r
		}
	}
}

// sortNodesByAddress gives a deterministic within-layer ordering.
func sortNodesByAddress(nodes []*Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Block < nodes[j].Block })
}
