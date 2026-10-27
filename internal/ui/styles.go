package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

// Text stays readable on every surface. Semantic colors identify changes;
// the orchid accent identifies focus, independently of the resource's action.
var (
	colCreate  = lipgloss.AdaptiveColor{Light: "#10734A", Dark: "#7DE2B1"}
	colUpdate  = lipgloss.AdaptiveColor{Light: "#885B00", Dark: "#F5CF82"}
	colDelete  = lipgloss.AdaptiveColor{Light: "#B52B49", Dark: "#FF8F9C"}
	colReplace = lipgloss.AdaptiveColor{Light: "#6C3ABE", Dark: "#C9ADFF"}
	colRead    = lipgloss.AdaptiveColor{Light: "#00659A", Dark: "#8CCBFF"}
	colMixed   = lipgloss.AdaptiveColor{Light: "#7A5A00", Dark: "#F1D58B"}
	colNoOp    = lipgloss.AdaptiveColor{Light: "#58617B", Dark: "#AFB5CA"}

	colDim    = lipgloss.AdaptiveColor{Light: "#4E566F", Dark: "#B6BDD3"}
	colText   = lipgloss.AdaptiveColor{Light: "#252536", Dark: "#EEF0FA"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#5D657B", Dark: "#9CA5C0"}
	colHeader = lipgloss.AdaptiveColor{Light: "#8639A6", Dark: "#E5A0F4"}

	colCanvas  = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111321"}
	colPanelBg = lipgloss.AdaptiveColor{Light: "#F5F5FA", Dark: "#1C1D2E"}
	colBorder  = lipgloss.AdaptiveColor{Light: "#A4A8B9", Dark: "#626781"}
	colSelText = lipgloss.AdaptiveColor{Light: "#252536", Dark: "#FFFFFF"}
	colSelBg   = lipgloss.AdaptiveColor{Light: "#EDE6FA", Dark: "#2D2848"}
	colKeyBg   = lipgloss.AdaptiveColor{Light: "#E6DDF2", Dark: "#39314D"}
	colArrow   = lipgloss.AdaptiveColor{Light: "#6E549D", Dark: "#B8A7EA"}
)

func actionColor(a tfplan.Action) lipgloss.TerminalColor {
	switch a {
	case tfplan.ActionCreate:
		return colCreate
	case tfplan.ActionUpdate:
		return colUpdate
	case tfplan.ActionDelete:
		return colDelete
	case tfplan.ActionReplace:
		return colReplace
	case tfplan.ActionRead:
		return colRead
	case tfplan.ActionMixed:
		return colMixed
	default:
		return colNoOp
	}
}

// Styles holds reusable lipgloss styles built once.
type Styles struct {
	add lipgloss.Style
	del lipgloss.Style
	upd lipgloss.Style

	dim    lipgloss.Style
	faint  lipgloss.Style
	text   lipgloss.Style
	accent lipgloss.Style
	header lipgloss.Style
	screen lipgloss.Style
	panel  lipgloss.Style
	key    lipgloss.Style
	badge  lipgloss.Style
}

func newStyles() Styles {
	return Styles{
		add:    lipgloss.NewStyle().Foreground(colCreate),
		del:    lipgloss.NewStyle().Foreground(colDelete),
		upd:    lipgloss.NewStyle().Foreground(colUpdate),
		dim:    lipgloss.NewStyle().Foreground(colDim),
		faint:  lipgloss.NewStyle().Foreground(colFaint),
		text:   lipgloss.NewStyle().Foreground(colText),
		accent: lipgloss.NewStyle().Foreground(colHeader).Bold(true),
		header: lipgloss.NewStyle().Foreground(colHeader).Bold(true),
		screen: lipgloss.NewStyle().Foreground(colText).Background(colCanvas),
		panel: lipgloss.NewStyle().Foreground(colText).Background(colPanelBg).
			Border(lipgloss.RoundedBorder()).BorderForeground(colBorder).Padding(0, 1),
		key:   lipgloss.NewStyle().Foreground(colHeader).Background(colKeyBg).Padding(0, 1).Bold(true),
		badge: lipgloss.NewStyle().Foreground(colText).Background(colPanelBg).Padding(0, 1),
	}
}

func (m Model) resourceCardStyle(width int, selected bool) lipgloss.Style {
	style := m.styles.panel.Width(max(1, width-2))
	if selected {
		style = style.Border(lipgloss.ThickBorder()).BorderForeground(colHeader).Background(colSelBg)
	}
	return style
}

// Lip Gloss v1's nested foreground styles reset the background too. Restore
// the enclosing surface after each child reset so panels do not develop holes.
func renderSurface(style lipgloss.Style, content string) string {
	if color := lipgloss.ColorProfile().FromColor(style.GetBackground()); color != nil {
		if sequence := color.Sequence(true); sequence != "" {
			content = strings.ReplaceAll(content, "\x1b[0m", "\x1b[0m\x1b["+sequence+"m")
		}
	}
	return style.Render(content)
}
