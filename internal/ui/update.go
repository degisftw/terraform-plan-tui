package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.ensureColVisible()
		if m.detailLines != nil {
			m.rebuildDetail()
		}
		return m, nil

	case tea.KeyMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateKeys(msg)
	}
	return m, nil
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.search.Blur()
		m.search.SetValue("")
		return m, nil
	case "enter":
		q := m.search.Value()
		m.searching = false
		m.search.Blur()
		var c, r int
		var ok bool
		if m.showStreams {
			var idx int
			c, r, idx, ok = m.searchStream(q)
			if ok {
				m.instanceIdx[m.g.At(c, r).Block] = idx
			}
		} else {
			c, r, ok = m.g.Search(q, m.col, m.row)
		}
		if ok {
			m.mode = modeGraph
			m.col, m.row = c, r
			m.snapSelection()
			m.ensureColVisible()
			m.searchMsg = ""
		} else {
			m.searchMsg = "no match for \"" + q + "\""
		}
		m.search.SetValue("")
		return m, nil
	}
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	return m, cmd
}

func (m Model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.detailLines != nil {
		return m.updateDetail(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc":
		if m.showHelp {
			m.showHelp = false
		}
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "a":
		m.toggleStreams()
		return m, nil

	case "/":
		m.searching = true
		m.searchMsg = ""
		m.search.Focus()
		return m, nil

	case "t":
		if m.mode == modeGraph {
			m.mode = modeList
			// align list selection with current graph node
			m.syncListToGraph()
		} else {
			m.mode = modeGraph
			m.syncGraphToList()
		}
		return m, nil

	case "u":
		m.showUnchanged = !m.showUnchanged
		return m, nil

	case "n":
		m.hideNoOp = !m.hideNoOp
		m.snapSelection()
		m.ensureColVisible()
		m.clampListSel()
		return m, nil

	case "x":
		// collapse everything
		m.expanded = map[string]bool{}
		return m, nil

	case "f":
		m.detailScroll = 0
		m.rebuildDetail()
		return m, nil

	case " ", "space", "enter":
		m.toggleExpand()
		return m, nil

	case "[", "shift+tab":
		m.cycleInstance(-1)
		return m, nil
	case "]", "tab":
		m.cycleInstance(1)
		return m, nil
	}

	if m.mode == modeList {
		return m.updateListNav(msg)
	}
	return m.updateGraphNav(msg)
}

// --- graph navigation -------------------------------------------------------

func (m Model) updateGraphNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.moveUp()
	case "down", "j":
		m.moveDown()
	case "left", "h":
		m.moveLeft()
	case "right", "l":
		m.moveRight()
	case "g", "home":
		if rows := m.visibleRows(m.col); len(rows) > 0 {
			m.row = rows[0]
			m.selectStreamEnd(false)
		}
	case "G", "end":
		if rows := m.visibleRows(m.col); len(rows) > 0 {
			m.row = rows[len(rows)-1]
			m.selectStreamEnd(true)
		}
	case "pgup":
		for i := 0; i < 5; i++ {
			m.moveUp()
		}
	case "pgdown":
		for i := 0; i < 5; i++ {
			m.moveDown()
		}
	}
	return m, nil
}

func (m *Model) moveUp() {
	if m.showStreams {
		n := m.g.At(m.col, m.row)
		if n != nil {
			instances := m.visibleInstanceIndices(n)
			if i := indexOf(instances, m.shownInstance(n)); i > 0 {
				m.instanceIdx[n.Block] = instances[i-1]
				return
			}
		}
	}
	rows := m.visibleRows(m.col)
	i := indexOf(rows, m.row)
	if i > 0 {
		m.row = rows[i-1]
		m.selectStreamEnd(true)
	}
}

func (m *Model) moveDown() {
	if m.showStreams {
		n := m.g.At(m.col, m.row)
		if n != nil {
			instances := m.visibleInstanceIndices(n)
			if i := indexOf(instances, m.shownInstance(n)); i >= 0 && i < len(instances)-1 {
				m.instanceIdx[n.Block] = instances[i+1]
				return
			}
		}
	}
	rows := m.visibleRows(m.col)
	i := indexOf(rows, m.row)
	if i >= 0 && i < len(rows)-1 {
		m.row = rows[i+1]
		m.selectStreamEnd(false)
	}
}

func (m *Model) moveLeft() {
	cur := m.g.At(m.col, m.row)
	if cur == nil {
		return
	}
	deps := cur.DepsOutSet()
	for c := m.col - 1; c >= 0; c-- {
		rows := m.visibleRows(c)
		if len(rows) == 0 {
			continue
		}
		target := -1
		for _, r := range rows {
			if deps[m.g.At(c, r).Block] {
				target = r
				break
			}
		}
		if target < 0 {
			target = nearestRow(rows, m.row)
		}
		m.col, m.row = c, target
		m.ensureColVisible()
		return
	}
}

func (m *Model) moveRight() {
	cur := m.g.At(m.col, m.row)
	if cur == nil {
		return
	}
	dependents := cur.DepsInSet()
	for c := m.col + 1; c <= m.g.MaxLayer(); c++ {
		rows := m.visibleRows(c)
		if len(rows) == 0 {
			continue
		}
		target := -1
		for _, r := range rows {
			if dependents[m.g.At(c, r).Block] {
				target = r
				break
			}
		}
		if target < 0 {
			target = nearestRow(rows, m.row)
		}
		m.col, m.row = c, target
		m.ensureColVisible()
		return
	}
}

// --- list navigation --------------------------------------------------------

func (m Model) updateListNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.listItems())
	switch msg.String() {
	case "up", "k":
		if m.listSel > 0 {
			m.listSel--
		}
	case "down", "j":
		if m.listSel < n-1 {
			m.listSel++
		}
	case "g", "home":
		m.listSel = 0
	case "G", "end":
		m.listSel = n - 1
	case "pgup":
		m.listSel -= 10
	case "pgdown":
		m.listSel += 10
	}
	if m.listSel < 0 {
		m.listSel = 0
	}
	if m.listSel > n-1 {
		m.listSel = n - 1
	}
	return m, nil
}

// --- shared actions ---------------------------------------------------------

func (m *Model) toggleExpand() {
	n := m.selectedNode()
	if n == nil {
		return
	}
	idx := -1
	if m.showStreams && m.mode == modeGraph {
		idx = m.shownInstance(n)
	}
	key := cardExpansionKey(n, idx)
	m.expanded[key] = !m.expanded[key]
}

func (m *Model) cycleInstance(delta int) {
	n := m.selectedNode()
	if n == nil || n.Count() <= 1 {
		return
	}
	if m.showStreams && m.hideNoOp {
		instances := m.visibleInstanceIndices(n)
		if len(instances) == 0 {
			return
		}
		pos := indexOf(instances, m.shownInstance(n))
		m.instanceIdx[n.Block] = instances[(pos+delta+len(instances))%len(instances)]
		return
	}
	idx := m.shownInstance(n) + delta
	if idx < 0 {
		idx = n.Count() - 1
	}
	if idx >= n.Count() {
		idx = 0
	}
	m.instanceIdx[n.Block] = idx
}

// clampListSel keeps the list-mode selection within the (possibly filtered) list.
func (m *Model) clampListSel() {
	n := len(m.listItems())
	if m.listSel >= n {
		m.listSel = n - 1
	}
	if m.listSel < 0 {
		m.listSel = 0
	}
}

// snapSelection makes sure the current graph selection points at a visible node.
func (m *Model) snapSelection() {
	if m.nodeVisible(m.g.At(m.col, m.row)) {
		return
	}
	if rows := m.visibleRows(m.col); len(rows) > 0 {
		m.row = nearestRow(rows, m.row)
		return
	}
	// current column empty; find nearest visible column.
	for d := 1; d <= m.g.MaxLayer()+1; d++ {
		for _, c := range []int{m.col - d, m.col + d} {
			if c < 0 || c > m.g.MaxLayer() {
				continue
			}
			if rows := m.visibleRows(c); len(rows) > 0 {
				m.col, m.row = c, rows[0]
				return
			}
		}
	}
	m.col, m.row = m.firstVisible()
}

func (m *Model) ensureColVisible() {
	vc := m.visibleCols()
	if m.col < m.colScroll {
		m.colScroll = m.col
	}
	if m.col > m.colScroll+vc-1 {
		m.colScroll = m.col - vc + 1
	}
	if m.colScroll < 0 {
		m.colScroll = 0
	}
}

// syncListToGraph / syncGraphToList keep the two modes pointing at the same node.
func (m *Model) syncListToGraph() {
	sel := m.g.At(m.col, m.row)
	if sel == nil {
		m.listSel = 0
		return
	}
	for i, n := range m.listItems() {
		if n.Block == sel.Block {
			m.listSel = i
			return
		}
	}
}

func (m *Model) syncGraphToList() {
	items := m.listItems()
	if m.listSel >= 0 && m.listSel < len(items) {
		if c, r, ok := m.g.Locate(items[m.listSel].Block); ok {
			m.col, m.row = c, r
			m.ensureColVisible()
		}
	}
}

func indexOf(rows []int, v int) int {
	for i, r := range rows {
		if r == v {
			return i
		}
	}
	return -1
}

func nearestRow(rows []int, want int) int {
	best := rows[0]
	bestDist := abs(rows[0] - want)
	for _, r := range rows[1:] {
		if d := abs(r - want); d < bestDist {
			best, bestDist = r, d
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
