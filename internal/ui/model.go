package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/omarismail/tfplantui/internal/tfplan"
)

type viewMode int

const (
	modeGraph viewMode = iota
	modeList
)

// Model is the Bubble Tea model for the plan explorer.
type Model struct {
	g      *tfplan.Graph
	styles Styles

	width, height int
	ready         bool

	mode viewMode

	// graph-mode selection + horizontal scroll
	col, row  int
	colScroll int

	// list-mode selection + scroll
	listSel    int
	listScroll int

	expanded      map[string]bool
	instanceIdx   map[string]int // block -> shown instance when expanded/multi
	showUnchanged bool
	hideNoOp      bool

	searching bool
	search    textinput.Model
	searchMsg string

	showHelp bool

	present []tfplan.Action // action icons that actually occur, for the legend
}

// New builds the model and selects the first visible node.
func New(g *tfplan.Graph) Model {
	ti := textinput.New()
	ti.Placeholder = "type part of a resource address, Enter to jump"
	ti.Prompt = "/"
	ti.CharLimit = 160

	m := Model{
		g:           g,
		styles:      newStyles(),
		mode:        modeGraph,
		expanded:    map[string]bool{},
		instanceIdx: map[string]int{},
		search:      ti,
		present:     presentActions(g),
	}
	m.col, m.row = m.firstVisible()
	return m
}

// presentActions returns, in canonical order, the action icons that actually
// occur in the plan — so the legend lists only what you'll see.
func presentActions(g *tfplan.Graph) []tfplan.Action {
	seen := map[tfplan.Action]bool{}
	for _, n := range g.Nodes {
		seen[n.ActionAgg()] = true
	}
	order := []tfplan.Action{
		tfplan.ActionCreate, tfplan.ActionReplace, tfplan.ActionDelete,
		tfplan.ActionUpdate, tfplan.ActionMixed, tfplan.ActionRead,
		tfplan.ActionForget, tfplan.ActionNoOp,
	}
	var out []tfplan.Action
	for _, a := range order {
		if seen[a] {
			out = append(out, a)
		}
	}
	return out
}

func (m Model) Init() tea.Cmd { return nil }

// --- visibility / filtering -------------------------------------------------

func (m Model) nodeVisible(n *tfplan.Node) bool {
	if n == nil {
		return false
	}
	if m.hideNoOp && n.ActionAgg() == tfplan.ActionNoOp {
		return false
	}
	return true
}

// visibleRows returns the row indices of visible nodes in a column.
func (m Model) visibleRows(col int) []int {
	n := m.g.ColLen(col)
	// Fast path: with no filter every row is visible, skip per-node checks.
	if !m.hideNoOp {
		rows := make([]int, n)
		for r := range rows {
			rows[r] = r
		}
		return rows
	}
	var rows []int
	for r := 0; r < n; r++ {
		if m.nodeVisible(m.g.At(col, r)) {
			rows = append(rows, r)
		}
	}
	return rows
}

// columnVisible reports whether a column has any visible node.
func (m Model) columnVisible(col int) bool {
	for r := 0; r < m.g.ColLen(col); r++ {
		if m.nodeVisible(m.g.At(col, r)) {
			return true
		}
	}
	return false
}

func (m Model) anyVisible() bool {
	for c := 0; c <= m.g.MaxLayer(); c++ {
		if m.columnVisible(c) {
			return true
		}
	}
	return false
}

func (m Model) firstVisible() (int, int) {
	for c := 0; c <= m.g.MaxLayer(); c++ {
		if rows := m.visibleRows(c); len(rows) > 0 {
			return c, rows[0]
		}
	}
	return 0, 0
}

// --- selection accessors ----------------------------------------------------

func (m Model) selectedNode() *tfplan.Node {
	if m.mode == modeList {
		items := m.listItems()
		if m.listSel >= 0 && m.listSel < len(items) {
			return items[m.listSel]
		}
		return nil
	}
	return m.g.At(m.col, m.row)
}

// shownInstance returns the instance index currently displayed for a block.
func (m Model) shownInstance(n *tfplan.Node) int {
	idx := m.instanceIdx[n.Block]
	if idx < 0 || idx >= n.Count() {
		return 0
	}
	return idx
}

// listItems returns the flattened, filtered node list for list mode,
// ordered by layer then address (roots first), matching reading order.
func (m Model) listItems() []*tfplan.Node {
	var items []*tfplan.Node
	for c := 0; c <= m.g.MaxLayer(); c++ {
		for r := 0; r < m.g.ColLen(c); r++ {
			n := m.g.At(c, r)
			if m.nodeVisible(n) {
				items = append(items, n)
			}
		}
	}
	return items
}

// --- layout helpers ---------------------------------------------------------

func (m Model) visibleCols() int {
	n := (m.width) / columnStride
	if n < 1 {
		n = 1
	}
	if n > m.g.MaxLayer()+1 {
		n = m.g.MaxLayer() + 1
	}
	return n
}

// bodyHeight is the rows available for the graph/list area.
func (m Model) bodyHeight() int {
	h := m.height - headerHeight - footerHeight
	if h < 3 {
		h = 3
	}
	return h
}
