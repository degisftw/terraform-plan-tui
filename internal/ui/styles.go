package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/tfplantui/internal/tfplan"
)

// Palette. Action colors carry all the meaning; everything else is neutral.
var (
	colCreate  = lipgloss.Color("#3FB950") // green  +  add
	colUpdate  = lipgloss.Color("#D29922") // amber  ~  update in place
	colDelete  = lipgloss.Color("#F85149") // red    -  destroy
	colReplace = lipgloss.Color("#BC8CFF") // purple ±  replace
	colRead    = lipgloss.Color("#58A6FF") // blue   ≤  read
	colMixed   = lipgloss.Color("#F2CC60") // yellow *  mixed count block
	colNoOp    = lipgloss.Color("#414850") // very dim — unimpacted resources recede

	colDim    = lipgloss.Color("#6E7681")
	colText   = lipgloss.Color("#C9D1D9")
	colFaint  = lipgloss.Color("#484F58")
	colHeader = lipgloss.Color("#79C0FF")

	// Selection: the interior fills dark and the label goes bright white, while the
	// border keeps its action color — the cursor reads by contrast, not by hue.
	colSelText = lipgloss.Color("#FFFFFF")
	colSelBg   = lipgloss.Color("#161B22")

	// Just-in-time dependency arrows drawn from the selected node.
	colArrow = lipgloss.Color("#768390")
)

func actionColor(a tfplan.Action) lipgloss.Color {
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
	accent lipgloss.Style // bright white, used for the cursor in lists
	header lipgloss.Style
}

func newStyles() Styles {
	return Styles{
		add:    lipgloss.NewStyle().Foreground(colCreate),
		del:    lipgloss.NewStyle().Foreground(colDelete),
		upd:    lipgloss.NewStyle().Foreground(colUpdate),
		dim:    lipgloss.NewStyle().Foreground(colDim),
		faint:  lipgloss.NewStyle().Foreground(colFaint),
		text:   lipgloss.NewStyle().Foreground(colText),
		accent: lipgloss.NewStyle().Foreground(colSelText).Bold(true),
		header: lipgloss.NewStyle().Foreground(colHeader).Bold(true),
	}
}
