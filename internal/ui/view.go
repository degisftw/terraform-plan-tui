package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

const (
	headerHeight = 3 // title · summary counts · icon legend
	footerHeight = 3
)

// View assembles the full screen: header, the active body (graph or list), and
// footer — unless help is showing, which takes over the whole screen.
func (m Model) View() string {
	if !m.ready {
		return "loading plan…"
	}
	if m.showHelp {
		return renderSurface(m.styles.screen, m.renderHelp())
	}

	var body string
	if m.detailLines != nil {
		body = m.renderDetail()
	} else if m.mode == modeList {
		body = m.renderList()
	} else {
		body = m.renderGraph()
	}

	return renderSurface(m.styles.screen, strings.Join([]string{
		m.renderHeader(),
		body,
		m.renderFooter(),
	}, "\n"))
}

// --- header -----------------------------------------------------------------

func (m Model) renderHeader() string {
	s := m.styles
	title := s.header.Bold(true).Render(" tfplantui ")
	path := m.g.PlanPath
	if path == "" {
		path = "plan"
	}
	meta := s.dim.Render("  " + path)
	if m.g.TerraformVersion != "" {
		meta += s.faint.Render("  ·  terraform " + m.g.TerraformVersion)
	}

	modeName := "GRAPH"
	if m.mode == modeList {
		modeName = "LIST"
	}
	flags := s.badge.Render(modeName)
	if m.detailLines != nil {
		flags += " " + s.key.Render("FOCUS")
	}
	if m.showStreams {
		flags += " " + s.badge.Render("ALL STREAMS")
	}
	if m.hideNoOp {
		flags += " " + s.badge.Render("changes only")
	}
	if m.showUnchanged {
		flags += " " + s.badge.Render("+unchanged")
	}
	left := clipANSI(title+meta, max(0, m.width-lipgloss.Width(flags)-2))
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(flags))
	line1 := left + strings.Repeat(" ", gap) + flags
	return padANSI(line1, m.width) + "\n" +
		padANSI(m.renderSummary(), m.width) + "\n" +
		padANSI(m.renderLegend(), m.width)
}

// renderLegend is a dense, single-line key for the icons actually present.
func (m Model) renderLegend() string {
	parts := make([]string, 0, len(m.present)+1)
	for _, a := range m.present {
		sym := lipgloss.NewStyle().Foreground(actionColor(a)).Bold(true).Render(a.Symbol())
		parts = append(parts, sym+" "+m.styles.dim.Render(legendWord(a)))
	}
	if m.showStreams {
		scope := m.styles.dim.Render("─▶ confirmed refs") + "    " + m.styles.faint.Render("? block scope")
		return clipANSI(scope+"    "+strings.Join(parts, "    "), m.width)
	}
	arrow := lipgloss.NewStyle().Foreground(colArrow).Render("─▶")
	parts = append(parts, arrow+" "+m.styles.dim.Render("depends on"))
	return clipANSI(strings.Join(parts, "    "), m.width)
}

// legendWord is the terse word shown for an action in the legend and help
// screen ("add" for create, "destroy" for delete, etc.).
func legendWord(a tfplan.Action) string {
	switch a {
	case tfplan.ActionCreate:
		return "add"
	case tfplan.ActionDelete:
		return "destroy"
	case tfplan.ActionReplace:
		return "replace"
	case tfplan.ActionUpdate:
		return "update"
	case tfplan.ActionMixed:
		return "mixed"
	case tfplan.ActionRead:
		return "read"
	case tfplan.ActionForget:
		return "forget"
	default:
		return "no-op"
	}
}

func (m Model) renderSummary() string {
	sm := m.g.Summary
	s := m.styles
	seg := func(style lipgloss.Style, n int, label string) string {
		if n == 0 {
			return ""
		}
		return style.Background(colPanelBg).Padding(0, 1).Bold(true).Render(fmt.Sprintf("%d %s", n, label)) + " "
	}
	out := seg(s.add, sm.Add, "to add") +
		seg(s.upd, sm.Change, "to change") +
		seg(s.del, sm.Destroy, "to destroy")
	if sm.Replace > 0 {
		out += lipgloss.NewStyle().Foreground(colReplace).Render(fmt.Sprintf("%d to replace", sm.Replace)) + "  "
	}
	if sm.NoOp > 0 {
		out += s.dim.Render(fmt.Sprintf("%d no-op", sm.NoOp)) + "  "
	}
	out += s.faint.Render(fmt.Sprintf("· %d instances / %d blocks", m.g.TotalInstances(), m.g.TotalNodes()))

	// Horizontal scroll indicator.
	if m.detailLines == nil && m.mode == modeGraph && m.g.MaxLayer()+1 > m.visibleCols() {
		last := m.colScroll + m.visibleCols()
		if last > m.g.MaxLayer()+1 {
			last = m.g.MaxLayer() + 1
		}
		out += s.faint.Render(fmt.Sprintf("   layers %d–%d of %d", m.colScroll, last-1, m.g.MaxLayer()))
	}
	return clipANSI(out, m.width)
}

// --- footer -----------------------------------------------------------------

func (m Model) renderFooter() string {
	rule := lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", m.width))
	ctx := m.renderContext()
	var bottom string
	switch {
	case m.searching:
		bottom = m.search.View()
	case m.searchMsg != "":
		bottom = m.styles.del.Render(m.searchMsg) + m.styles.faint.Render("   (press / to search again)")
	default:
		bottom = m.renderHints()
	}
	bar := renderSurface(m.styles.screen.Background(colPanelBg), padANSI(bottom, m.width))
	return padANSI(rule, m.width) + "\n" + padANSI(ctx, m.width) + "\n" + bar
}

func (m Model) renderContext() string {
	n := m.selectedNode()
	if n == nil {
		return m.styles.dim.Render("no selection")
	}
	act := n.ActionAgg()
	label := n.Label()
	if m.showStreams && n.Count() > 0 {
		inst := n.Instances[m.shownInstance(n)]
		act, label = inst.Action, inst.Address()
	}
	sym := lipgloss.NewStyle().Foreground(actionColor(act)).Bold(true).Render(act.Symbol())
	word := lipgloss.NewStyle().Foreground(actionColor(act)).Render(act.Label())
	addr := m.styles.text.Bold(true).Render(label)
	if !m.showStreams && n.Count() > 1 {
		addr += m.styles.dim.Render(" ×" + strconv.Itoa(n.Count()))
	}
	out := sym + " " + addr + "  " + word
	if m.showStreams && n.Count() > 0 {
		up, upGroups := m.streamRelated(n, m.shownInstance(n), true)
		down, downGroups := m.streamRelated(n, m.shownInstance(n), false)
		out += m.streamContextLinks("depends on", up, upGroups)
		out += m.streamContextLinks("used by", down, downGroups)
	} else {
		if len(n.DepsOut) > 0 {
			out += m.styles.faint.Render("   depends on ") + m.styles.dim.Render(shortList(n.DepsOut, 3))
		}
		if len(n.DepsIn) > 0 {
			out += m.styles.faint.Render("   used by ") + m.styles.dim.Render(shortList(n.DepsIn, 3))
		}
	}
	idx := -1
	if m.showStreams && m.mode == modeGraph {
		idx = m.shownInstance(n)
	}
	if m.detailLines == nil && !m.expanded[cardExpansionKey(n, idx)] {
		out += m.styles.faint.Render("   · space to expand")
	}
	return out
}

func (m Model) renderHints() string {
	shortcut := func(key, label string) string {
		return m.styles.key.Render(key) + " " + m.styles.dim.Render(label)
	}
	tail := shortcut("?", "help") + "  " + shortcut("q", "quit")
	if m.width <= lipgloss.Width(tail) {
		return clipANSI(tail, m.width)
	}
	bindings := [][2]string{{"f", "inspect"}, {"a", "streams"}, {"↑↓←→", "move"}, {"Tab/⇧Tab", "instance"}, {"Space", "preview"}, {"/", "search"}, {"t", "list"}}
	if m.mode == modeList {
		bindings[2] = [2]string{"↑↓", "move"}
		bindings[6][1] = "graph"
	}
	if m.detailLines != nil {
		bindings = [][2]string{{"Esc", "back"}, {"a", "streams"}, {"↑↓", "scroll"}, {"←→", "levels"}, {"Tab/⇧Tab", "instance"}, {"PgUp/Dn", "page"}, {"u", "unchanged"}}
	}
	n := m.selectedNode()
	available := m.width - lipgloss.Width(tail) - 2
	var b strings.Builder
	used := 0
	for _, binding := range bindings {
		if binding[1] == "instance" && (n == nil || n.Count() <= 1) {
			continue
		}
		item := shortcut(binding[0], binding[1])
		w := lipgloss.Width(item)
		if used+w > available {
			continue
		}
		b.WriteString(item)
		b.WriteString("  ")
		used += w + 2
	}
	b.WriteString(strings.Repeat(" ", max(0, m.width-used-lipgloss.Width(tail))))
	b.WriteString(tail)
	return b.String()
}

// --- helpers ----------------------------------------------------------------

func (m Model) emptyMessage() string {
	if m.g.TotalNodes() == 0 {
		return "This plan proposes no resource changes — infrastructure is up to date."
	}
	if m.hideNoOp {
		return "No changes to show. Press  n  to include no-op resources."
	}
	return "Nothing to display."
}
