package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

// detailWidth reserves room for a compact neighboring column on each side.
// On narrower terminals the neighbors shrink before the focused card does.
func (m Model) detailWidth() int {
	side := min(cardOuter, max(0, (m.width-(cardOuter+16)-2*columnGap)/2))
	if side == 0 {
		return max(1, m.width)
	}
	return min(100, m.width-2*(side+columnGap))
}

func (m Model) detailHeight() int {
	return max(1, m.bodyHeight()-4) // layer header, borders, and scroll position
}

func (m *Model) clampDetailScroll() {
	m.detailScroll = max(0, min(m.detailScroll, len(m.detailLines)-m.detailHeight()))
}

func (m *Model) rebuildDetail() {
	n := m.selectedNode()
	if n == nil || !m.nodeVisible(n) || n.Count() == 0 {
		m.detailLines = nil
		return
	}
	inst := n.Instances[m.shownInstance(n)]
	width := max(1, m.detailWidth()-4)
	wrap := lipgloss.NewStyle().Width(width)
	m.detailLines = strings.Split(wrap.Render(m.styles.text.Bold(true).Render(inst.Address())), "\n")
	status := m.styles.badge.Foreground(actionColor(inst.Action)).Bold(true).Render(inst.Action.Symbol() + " " + inst.Action.Label())
	info := status + m.styles.dim.Render(fmt.Sprintf("  instance %d/%d", m.shownInstance(n)+1, n.Count()))
	m.detailLines = append(m.detailLines, strings.Split(wrap.Render(info), "\n")...)
	m.detailLines = append(m.detailLines, "")
	if m.showStreams {
		m.detailLines = append(m.detailLines, m.streamReferenceLines(n, width)...)
		m.detailLines = append(m.detailLines, "")
	}

	// Use the existing diff, including its sensitivity and unknown-value masks.
	diff := inst.Diff(m.showUnchanged)
	if len(diff) == 0 {
		m.detailLines = append(m.detailLines, strings.Split(wrap.Render(m.styles.dim.Render("(no attribute changes; press u to show unchanged attributes)")), "\n")...)
	}
	for _, line := range diff {
		m.detailLines = append(m.detailLines, renderDetailDiffLine(line, width)...)
	}
	m.clampDetailScroll()
}

// renderDetail keeps the enlarged card in the graph, flanked by the nearest
// visible dependency layers. Only the focused card's contents scroll.
func (m Model) renderDetail() string {
	n := m.selectedNode()
	width := m.detailWidth()
	height := m.bodyHeight() - 1
	end := min(m.detailScroll+m.detailHeight(), len(m.detailLines))
	content := make([]string, end-m.detailScroll+1)
	copy(content, m.detailLines[m.detailScroll:end])
	info := fmt.Sprintf("lines %d–%d of %d · instance %d/%d", m.detailScroll+1, end, len(m.detailLines), m.shownInstance(n)+1, n.Count())
	content[len(content)-1] = m.styles.dim.Render(clip(info, max(1, width-4)))
	box := renderSurface(m.resourceCardStyle(width, true), strings.Join(content, "\n"))
	boxLines := strings.Split(box, "\n")
	top := max(0, (height-len(boxLines))/2)
	centerY := min(height-1, top+len(boxLines)/2)
	band := max(0, (m.width-width)/2)
	gap := min(columnGap, band)
	sideWidth := min(cardOuter, band-gap)
	margin := strings.Repeat(" ", band-gap-sideWidth)

	leftCol := m.detailNeighborColumn(n.Layer, -1)
	rightCol := m.detailNeighborColumn(n.Layer, 1)
	left, leftRows := m.renderDetailNeighbor(leftCol, sideWidth, height, centerY, n.DepsOutSet())
	right, rightRows := m.renderDetailNeighbor(rightCol, sideWidth, height, centerY, n.DepsInSet())
	leftGap, rightGap := make([]string, height), make([]string, height)
	space := strings.Repeat(" ", gap)
	for i := range height {
		leftGap[i], rightGap[i] = space, space
	}
	if gap > 0 && len(leftRows) > 0 {
		leftGap = gapConnector(gap, height, leftRows, []int{centerY}, map[int]bool{centerY: true})
	}
	if gap > 0 && len(rightRows) > 0 {
		arrows := make(map[int]bool, len(rightRows))
		for _, y := range rightRows {
			arrows[y] = true
		}
		rightGap = gapConnector(gap, height, []int{centerY}, rightRows, arrows)
	}
	header := func(col, w int) string {
		if col < 0 {
			return strings.Repeat(" ", w)
		}
		return m.styles.dim.Bold(true).Render(centerPad(fmt.Sprintf("Layer %d", col), w))
	}
	lines := make([]string, 0, m.bodyHeight())
	lines = append(lines, margin+header(leftCol, sideWidth)+space+header(n.Layer, width)+space+header(rightCol, sideWidth)+margin)
	blankMiddle := strings.Repeat(" ", width)
	for i := range height {
		middle := blankMiddle
		if j := i - top; j >= 0 && j < len(boxLines) {
			middle = boxLines[j]
		}
		lines = append(lines, margin+left[i]+leftGap[i]+middle+rightGap[i]+right[i]+margin)
	}
	return padLines(lines, m.bodyHeight(), m.width)
}

func (m Model) detailNeighborColumn(col, delta int) int {
	for c := col + delta; c >= 0 && c <= m.g.MaxLayer(); c += delta {
		if m.columnVisible(c) {
			return c
		}
	}
	return -1
}

func (m Model) renderDetailNeighbor(col, width, height, centerY int, related map[string]bool) ([]string, []int) {
	if col < 0 || width < 6 {
		blank := make([]string, height)
		space := strings.Repeat(" ", width)
		for i := range blank {
			blank[i] = space
		}
		return blank, nil
	}
	if m.showStreams {
		n := m.selectedNode()
		confirmed, grouped := m.streamRelated(n, m.shownInstance(n), col < n.Layer)
		return m.renderStreamDetailNeighbor(col, width, height, centerY, confirmed, grouped)
	}
	rows := m.visibleRows(col)
	target := rows[0]
	for _, r := range rows {
		if related[m.g.At(col, r).Block] {
			target = r
			break
		}
	}
	// Negative offsets add blank space above short columns, aligning the
	// nearest related card with the enlarged card rather than the top edge.
	centers := m.columnCenters(col, 0)
	scroll := centers[target] - centerY
	var connected []int
	for r, y := range centers {
		y -= scroll
		if related[m.g.At(col, r).Block] && y >= 0 && y < height {
			connected = append(connected, y)
		}
	}
	return m.renderColumnWindow(col, nil, scroll, height, width), connected
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Help is an overlay: dismiss it without changing the resource or scroll.
	if m.showHelp {
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?", "esc":
			m.showHelp = false
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "f", "esc":
		m.detailLines = nil
		m.detailScroll = 0
	case "?":
		m.showHelp = true
	case "a":
		m.toggleStreams()
	case "up", "k":
		m.detailScroll--
	case "down", "j":
		m.detailScroll++
	case "pgup", "ctrl+u":
		m.detailScroll -= m.detailHeight()
	case "pgdown", "ctrl+d":
		m.detailScroll += m.detailHeight()
	case "g", "home":
		m.detailScroll = 0
	case "G", "end":
		m.detailScroll = len(m.detailLines)
	case "u":
		m.showUnchanged = !m.showUnchanged
		m.rebuildDetail()
	case "left", "right", "h", "l":
		if m.mode == modeList {
			m.syncGraphToList()
		}
		col, row := m.col, m.row
		switch msg.String() {
		case "left", "h":
			m.moveLeft()
		default:
			m.moveRight()
		}
		if m.col != col || m.row != row {
			if m.mode == modeList {
				m.syncListToGraph()
			}
			m.detailScroll = 0
			m.rebuildDetail()
		}
	case "[", "]", "tab", "shift+tab":
		delta := 1
		switch msg.String() {
		case "[", "shift+tab":
			delta = -1
		}
		m.cycleInstance(delta)
		m.detailScroll = 0
		m.rebuildDetail()
	}
	m.clampDetailScroll()
	return m, nil
}

// Unlike card previews, detail rows wrap instead of truncating. Continuations
// align with the attribute text, and collections are indented before wrapping.
func renderDetailDiffLine(line tfplan.DiffLine, width int) []string {
	line.Before = detailValue(line.Before)
	line.After = detailValue(line.After)
	body := diffLineBody(line)
	if line.Forces {
		body += lipgloss.NewStyle().Foreground(colReplace).Render(" # forces replacement")
	}
	symbol := lipgloss.NewStyle().Foreground(diffKindColor(line.Kind)).Bold(true).Render(diffKindSymbol(line.Kind))
	prefix := strings.Repeat(" ", min(line.Indent*2, max(0, width-3))) + symbol + " "
	// Very narrow terminals still need to show the value, not just its prefix.
	if lipgloss.Width(prefix) >= width {
		prefix = ""
	}
	prefixWidth := lipgloss.Width(prefix)
	wrapped := lipgloss.NewStyle().Foreground(colText).Width(max(1, width-prefixWidth)).Render(body)
	lines := strings.Split(wrapped, "\n")
	continuation := strings.Repeat(" ", prefixWidth)
	for i, text := range lines {
		if i == 0 {
			lines[i] = prefix + text
		} else {
			lines[i] = continuation + text
		}
	}
	return lines
}

func detailValue(value string) string {
	if !strings.HasPrefix(value, "[") && !strings.HasPrefix(value, "{") {
		return value
	}
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(value), "", "  "); err != nil {
		return value
	}
	return out.String()
}
