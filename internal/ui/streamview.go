package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

// cardExpansionKey keeps compact previews block-scoped and stream previews
// instance-scoped. The index is the original index into Node.Instances.
func cardExpansionKey(n *tfplan.Node, idx int) string {
	if idx < 0 {
		return n.Block
	}
	return n.Instances[idx].Address()
}

func (m Model) visibleInstanceIndices(n *tfplan.Node) []int {
	indices := make([]int, 0, n.Count())
	for idx, inst := range n.Instances {
		if !m.hideNoOp || inst.Action != tfplan.ActionNoOp {
			indices = append(indices, idx)
		}
	}
	return indices
}

type streamCard struct {
	node   *tfplan.Node
	idx    int
	y      int
	height int
}

func (m Model) streamColumnCards(c int) []streamCard {
	return m.streamColumnCardsWidth(c, cardOuter)
}

func (m Model) streamColumnCardsWidth(c, width int) []streamCard {
	var cards []streamCard
	y := 0
	for _, row := range m.visibleRows(c) {
		n := m.g.At(c, row)
		for _, idx := range m.visibleInstanceIndices(n) {
			h := m.streamCardHeightWidth(n, idx, width)
			cards = append(cards, streamCard{node: n, idx: idx, y: y, height: h})
			y += h
		}
	}
	return cards
}

// streamInstanceCenters retains instance identity for detail-neighbor routing.
func (m Model) streamInstanceCenters(col, lineScroll int) map[string]int {
	centers := map[string]int{}
	for _, card := range m.streamColumnCards(col) {
		centers[card.node.Instances[card.idx].Address()] = card.y - lineScroll + card.height/2
	}
	return centers
}

// Stream labels and annotations wrap rather than truncate: every full address
// remains recoverable even when the shared connector rail merges several edges.
func streamWrap(text string, width int) []string {
	return strings.Split(ansi.Hardwrap(text, max(1, width), true), "\n")
}

func streamWrappedHeight(text string, width int) int {
	width = max(1, width)
	for _, r := range text {
		if r > 127 {
			return len(streamWrap(text, width))
		}
	}
	return max(1, (len(text)+width-1)/width)
}

func (m Model) streamCardContent(n *tfplan.Node, idx int, selected bool, width int) []string {
	inst := n.Instances[idx]
	textW := max(1, width-4)
	labelStyle := m.styles.text
	if selected {
		labelStyle = lipgloss.NewStyle().Foreground(colSelText).Bold(true)
	} else if inst.Action == tfplan.ActionNoOp {
		labelStyle = m.styles.faint
	}
	label := streamWrap(inst.Address(), max(1, textW-2))
	sym := lipgloss.NewStyle().Foreground(actionColor(inst.Action)).Bold(true).Render(inst.Action.Symbol())
	lines := []string{sym + " " + labelStyle.Render(label[0])}
	for _, part := range label[1:] {
		lines = append(lines, "  "+labelStyle.Render(part))
	}
	lines = append(lines, m.streamReferenceLines(n, textW)...)
	if m.expanded[cardExpansionKey(n, idx)] {
		diff := inst.Diff(m.showUnchanged)
		if len(diff) == 0 {
			lines = append(lines, m.styles.dim.Render("(no attribute changes)"))
		}
		for _, line := range diff {
			lines = append(lines, renderDiffLine(line, textW))
		}
	}
	for i, line := range lines {
		lines[i] = clipANSI(line, textW)
	}
	return lines
}

func (m Model) streamReferenceLines(n *tfplan.Node, width int) []string {
	var lines []string
	for _, dep := range n.InstanceDeps {
		for _, address := range dep.Instances {
			if !m.streamAddressVisible(dep.Block, address) {
				continue
			}
			for _, part := range streamWrap("confirmed upstream: "+address, width) {
				lines = append(lines, m.styles.dim.Render(part))
			}
		}
		if dep.Unresolved {
			for _, part := range streamWrap("? block scope: "+dep.Block, width) {
				lines = append(lines, m.styles.faint.Render(part))
			}
		}
	}
	return lines
}

func (m Model) streamAddressVisible(block, address string) bool {
	if !m.hideNoOp {
		return true
	}
	n := m.g.ByBlock[block]
	if n == nil {
		return !m.hideNoOp
	}
	for _, inst := range n.Instances {
		if inst.Address() == address {
			return !m.hideNoOp || inst.Action != tfplan.ActionNoOp
		}
	}
	return !m.hideNoOp
}

func (m Model) streamCardHeight(n *tfplan.Node, idx int) int {
	return m.streamCardHeightWidth(n, idx, cardOuter)
}

func (m Model) streamCardHeightWidth(n *tfplan.Node, idx, width int) int {
	textW := max(1, width-4)
	height := 2 + streamWrappedHeight(n.Instances[idx].Address(), max(1, textW-2))
	for _, dep := range n.InstanceDeps {
		for _, address := range dep.Instances {
			if m.streamAddressVisible(dep.Block, address) {
				height += streamWrappedHeight("confirmed upstream: "+address, textW)
			}
		}
		if dep.Unresolved {
			height += streamWrappedHeight("? block scope: "+dep.Block, textW)
		}
	}
	if m.expanded[cardExpansionKey(n, idx)] {
		height += max(1, len(n.Instances[idx].Diff(m.showUnchanged)))
	}
	return height
}

func (m Model) renderStreamColumnWindow(c int, sel *tfplan.Node, scroll, height, width int) []string {
	buf := make([]string, height)
	blank := strings.Repeat(" ", width)
	for i := range buf {
		buf[i] = blank
	}
	for _, card := range m.streamColumnCardsWidth(c, width) {
		if card.y+card.height <= scroll {
			continue
		}
		if card.y >= scroll+height {
			break
		}
		selected := card.node == sel && card.idx == m.shownInstance(card.node)
		content := m.streamCardContent(card.node, card.idx, selected, width)
		box := renderSurface(m.resourceCardStyle(width, selected), strings.Join(content, "\n"))
		for j, line := range strings.Split(box, "\n") {
			if pos := card.y + j - scroll; pos >= 0 && pos < height {
				buf[pos] = line
			}
		}
	}
	return buf
}

// Only confirmed instance references enter a rail. Unknown relationships stay
// grouped by upstream block in the explicitly marked card annotations; they are
// never expanded into invented instance-to-instance edges.
func (m Model) streamGap(left, right []streamCard, scroll, height int) []string {
	upstream := make(map[string]int, len(left))
	for _, card := range left {
		y := card.y - scroll + card.height/2
		if y >= 0 && y < height {
			upstream[card.node.Instances[card.idx].Address()] = y
		}
	}
	var from, to []int
	arrows := map[int]bool{}
	for _, card := range right {
		y := card.y - scroll + card.height/2
		if y < 0 || y >= height {
			continue
		}
		for _, dep := range card.node.InstanceDeps {
			for _, address := range dep.Instances {
				if source, ok := upstream[address]; ok {
					from = append(from, source)
					to = append(to, y)
					arrows[y] = true
				}
			}
		}
	}
	return gapConnector(columnGap, height, from, to, arrows)
}

func (m Model) renderStreamGraph() string {
	bodyH := m.bodyHeight()
	height := max(1, bodyH-1)
	if !m.anyVisible() {
		return centerMessage(m.emptyMessage(), bodyH, m.width)
	}
	start := m.colScroll
	end := min(start+m.visibleCols(), m.g.MaxLayer()+1)
	scroll := m.graphLineScroll(height)
	sel := m.g.At(m.col, m.row)
	columns := make([][]streamCard, end-start)
	buffers := make([][]string, end-start)
	gaps := make([][]string, max(0, end-start-1))
	headers := make([]string, end-start)
	for c := start; c < end; c++ {
		i := c - start
		columns[i] = m.streamColumnCards(c)
		headers[i] = m.styles.dim.Bold(true).Render(centerPad(fmt.Sprintf("Layer %d · %d instances", c, len(columns[i])), cardOuter))
		buffers[i] = m.renderColumnWindow(c, sel, scroll, height, cardOuter)
		if i > 0 {
			gaps[i-1] = m.streamGap(columns[i-1], columns[i], scroll, height)
		}
	}
	blank := strings.Repeat(" ", columnGap)
	out := []string{strings.Join(headers, blank)}
	for y := range height {
		var line strings.Builder
		for c, buf := range buffers {
			line.WriteString(buf[y])
			if c < len(gaps) {
				if gaps[c] == nil {
					line.WriteString(blank)
				} else {
					line.WriteString(gaps[c][y])
				}
			}
		}
		out = append(out, line.String())
	}
	return padLines(out, bodyH, m.width)
}
