package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/tfplantui/internal/tfplan"
)

// Layout constants for the columnar graph.
const (
	cardInner    = 30 // lipgloss Width of a card box (content + padding)
	cardOuter    = cardInner + 2
	columnGap    = 3 // wide enough to route dependency arrows
	columnStride = cardOuter + columnGap
)

// renderGraph lays out the dependency graph as columns of cards, windowed to the
// visible band, with just-in-time dependency arrows drawn beside the selection.
func (m Model) renderGraph() string {
	bodyH := m.bodyHeight()
	cardsH := bodyH - 1
	if cardsH < 1 {
		cardsH = 1
	}
	if !m.anyVisible() {
		return centerMessage(m.emptyMessage(), bodyH, m.width)
	}
	sel := m.g.At(m.col, m.row)

	vc := m.visibleCols()
	startCol := m.colScroll
	endCol := startCol + vc
	if endCol > m.g.MaxLayer()+1 {
		endCol = m.g.MaxLayer() + 1
	}

	// Vertical scroll is driven by the focused column's selected card; the same
	// offset is applied to every column so connected nodes stay roughly aligned.
	lineScroll := m.graphLineScroll(cardsH)
	spaces := strings.Repeat(" ", columnGap)

	// Column header row (sticky).
	var headerCells []string
	for c := startCol; c < endCol; c++ {
		label := fmt.Sprintf("Layer %d", c)
		if n := len(m.visibleRows(c)); n != m.g.ColLen(c) {
			label += fmt.Sprintf(" · %d/%d", n, m.g.ColLen(c))
		} else {
			label += fmt.Sprintf(" · %d", n)
		}
		headerCells = append(headerCells, m.styles.faint.Render(centerPad(label, cardOuter)))
	}
	out := []string{strings.Join(headerCells, spaces)}

	// Each column rendered windowed: only cards intersecting the visible band.
	bufs := make([][]string, 0, endCol-startCol)
	for c := startCol; c < endCol; c++ {
		bufs = append(bufs, m.renderColumnWindow(c, sel, lineScroll, cardsH))
	}

	// Just-in-time dependency arrows from the selected node into the gaps beside it.
	leftGap, rightGap := m.dependencyArrows(sel, startCol, endCol, lineScroll, cardsH)
	selVis := m.col - startCol

	for i := 0; i < cardsH; i++ {
		var sb strings.Builder
		for j, buf := range bufs {
			sb.WriteString(buf[i])
			if j == len(bufs)-1 {
				break
			}
			switch {
			case leftGap != nil && j == selVis-1:
				sb.WriteString(leftGap[i])
			case rightGap != nil && j == selVis:
				sb.WriteString(rightGap[i])
			default:
				sb.WriteString(spaces)
			}
		}
		out = append(out, sb.String())
	}
	return padLines(out, bodyH, m.width)
}

// columnCenters maps each visible row in a column to the screen-Y of its card's
// vertical center (within the cards area; may fall outside [0,cardsH)).
func (m Model) columnCenters(c, lineScroll int) map[int]int {
	centers := map[int]int{}
	y := 0
	for _, r := range m.visibleRows(c) {
		h := m.cardHeight(m.g.At(c, r))
		centers[r] = y - lineScroll + h/2
		y += h
	}
	return centers
}

// dependencyArrows builds the connector strings for the gaps immediately left and
// right of the selected node: left = arrows from its dependencies (in col-1) into
// it, right = arrows from it out to its dependents (in col+1). Neighbours in
// non-adjacent columns or scrolled off-screen are not drawn (the footer lists them).
func (m Model) dependencyArrows(sel *tfplan.Node, startCol, endCol, lineScroll, cardsH int) (left, right []string) {
	if sel == nil {
		return nil, nil
	}
	wantLeft := m.col-1 >= startCol && len(sel.DepsOut) > 0
	wantRight := m.col+1 < endCol && len(sel.DepsIn) > 0
	if !wantLeft && !wantRight {
		return nil, nil
	}
	yS, ok := m.columnCenters(m.col, lineScroll)[m.row]
	if !ok {
		return nil, nil
	}
	if yS < 0 {
		yS = 0
	}
	if yS >= cardsH {
		yS = cardsH - 1
	}

	if wantLeft {
		deps := sel.DepsOutSet()
		centers := m.columnCenters(m.col-1, lineScroll)
		var rows []int
		for _, r := range m.visibleRows(m.col - 1) {
			if deps[m.g.At(m.col-1, r).Block] {
				if y, ok := centers[r]; ok && y >= 0 && y < cardsH {
					rows = append(rows, y)
				}
			}
		}
		if len(rows) > 0 {
			left = gapConnector(columnGap, cardsH, rows, []int{yS}, map[int]bool{yS: true})
		}
	}
	if wantRight {
		dependents := sel.DepsInSet()
		centers := m.columnCenters(m.col+1, lineScroll)
		var rows []int
		arrow := map[int]bool{}
		for _, r := range m.visibleRows(m.col + 1) {
			if dependents[m.g.At(m.col+1, r).Block] {
				if y, ok := centers[r]; ok && y >= 0 && y < cardsH {
					rows = append(rows, y)
					arrow[y] = true
				}
			}
		}
		if len(rows) > 0 {
			right = gapConnector(columnGap, cardsH, []int{yS}, rows, arrow)
		}
	}
	return left, right
}

// gapConnector draws a vertical-rail connector across a column gap of width G.
// leftRows get a stub to the left edge, rightRows to the right edge, and rows in
// arrowRows end in a ▶ at the right edge. Returns cardsH styled strings of width G.
func gapConnector(G, cardsH int, leftRows, rightRows []int, arrowRows map[int]bool) []string {
	rail := G / 2
	leftSet := map[int]bool{}
	rightSet := map[int]bool{}
	ymin, ymax := 1<<30, -1
	note := func(y int) {
		if y < ymin {
			ymin = y
		}
		if y > ymax {
			ymax = y
		}
	}
	for _, y := range leftRows {
		leftSet[y] = true
		note(y)
	}
	for _, y := range rightRows {
		rightSet[y] = true
		note(y)
	}
	if ymax < 0 {
		return nil
	}

	style := lipgloss.NewStyle().Foreground(colArrow)
	blank := strings.Repeat(" ", G)
	lines := make([]string, cardsH)
	for y := 0; y < cardsH; y++ {
		if y < ymin || y > ymax {
			lines[y] = blank
			continue
		}
		run := []rune(blank)
		l, r := leftSet[y], rightSet[y]
		run[rail] = junctionChar(y > ymin, y < ymax, l, r)
		if l {
			for x := 0; x < rail; x++ {
				run[x] = '─'
			}
		}
		if r {
			for x := rail + 1; x < G; x++ {
				run[x] = '─'
			}
			if arrowRows[y] {
				run[G-1] = '▶'
			}
		}
		lines[y] = style.Render(string(run))
	}
	return lines
}

func junctionChar(up, down, left, right bool) rune {
	switch {
	case up && down && left && right:
		return '┼'
	case up && down && right:
		return '├'
	case up && down && left:
		return '┤'
	case down && left && right:
		return '┬'
	case up && left && right:
		return '┴'
	case up && down:
		return '│'
	case down && right:
		return '╭'
	case down && left:
		return '╮'
	case up && right:
		return '╰'
	case up && left:
		return '╯'
	case up || down:
		return '│'
	default:
		return '─'
	}
}

// graphLineScroll computes the vertical line offset that keeps the selected
// card visible within the focused column.
func (m Model) graphLineScroll(cardsH int) int {
	vrows := m.visibleRows(m.col)
	vi := indexOf(vrows, m.row)
	if vi < 0 {
		return 0
	}
	selStart, total := 0, 0
	var selH int
	for i, r := range vrows {
		h := m.cardHeight(m.g.At(m.col, r))
		if i == vi {
			selStart = total
			selH = h
		}
		total += h
	}
	return scrollFor(selStart, selH, total, cardsH)
}

// renderColumnWindow returns exactly cardsH lines for one column, rendering only
// the cards whose vertical extent intersects [lineScroll, lineScroll+cardsH).
func (m Model) renderColumnWindow(c int, sel *tfplan.Node, lineScroll, cardsH int) []string {
	buf := make([]string, cardsH)
	blank := strings.Repeat(" ", cardOuter)
	for i := range buf {
		buf[i] = blank
	}
	y := 0
	for _, r := range m.visibleRows(c) {
		n := m.g.At(c, r)
		h := m.cardHeight(n)
		if y+h <= lineScroll { // entirely above the window
			y += h
			continue
		}
		if y >= lineScroll+cardsH { // entirely below the window
			break
		}
		for j, line := range strings.Split(m.renderCardBox(n, sel), "\n") {
			if pos := y + j - lineScroll; pos >= 0 && pos < cardsH {
				buf[pos] = line
			}
		}
		y += h
	}
	return buf
}

// cardContentLines builds the inner (border-free) lines of a card. The line
// COUNT does not depend on `selected` (which only bolds the label), so it is
// safe to use for height measurement.
func (m Model) cardContentLines(n *tfplan.Node, selected bool) []string {
	act := n.ActionAgg()
	textW := cardInner - 2 // padding 1 on each side

	badge := ""
	if n.Count() > 1 {
		badge = " ×" + strconv.Itoa(n.Count())
	}
	avail := textW - 2 - len([]rune(badge))
	if avail < 1 {
		avail = 1
	}
	label := clip(n.Label(), avail)
	// Three-tier emphasis: selected = bright white; no-op = recedes; else normal.
	labelStyle := m.styles.text
	switch {
	case selected:
		labelStyle = lipgloss.NewStyle().Foreground(colSelText).Bold(true)
	case act == tfplan.ActionNoOp:
		labelStyle = m.styles.faint
	}
	sym := lipgloss.NewStyle().Foreground(actionColor(act)).Bold(true).Render(act.Symbol())
	lines := []string{sym + " " + labelStyle.Render(label) + m.styles.faint.Render(badge)}

	if m.expanded[n.Block] {
		idx := m.shownInstance(n)
		inst := n.Instances[idx]
		if n.Count() > 1 {
			info := fmt.Sprintf("instance %d/%d %s  [ ]", idx+1, n.Count(), inst.Action.Symbol())
			lines = append(lines, m.styles.faint.Render(clip(info, textW)))
		}
		diff := inst.Diff(m.showUnchanged)
		if len(diff) == 0 {
			lines = append(lines, m.styles.dim.Render("(no attribute changes)"))
		}
		for _, l := range diff {
			lines = append(lines, renderDiffLine(l, textW))
		}
	}
	return lines
}

func (m Model) cardHeight(n *tfplan.Node) int {
	return len(m.cardContentLines(n, false)) + 2 // + top & bottom border
}

func (m Model) renderCardBox(n, sel *tfplan.Node) string {
	selected := n == sel
	// The border and the action symbol always keep their action color. Selection
	// is shown only by filling the interior dark and brightening the label — the
	// boundary hue never changes as the cursor moves.
	style := lipgloss.NewStyle().
		Width(cardInner).
		Padding(0, 1).
		BorderForeground(actionColor(n.ActionAgg()))
	if selected {
		style = style.Border(lipgloss.ThickBorder()).Background(colSelBg)
	} else {
		style = style.Border(lipgloss.RoundedBorder())
	}
	return style.Render(strings.Join(m.cardContentLines(n, selected), "\n"))
}
