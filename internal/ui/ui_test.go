package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")

func plain(s string) string { return ansiRe.ReplaceAllString(s, "") }

func loadUI(t *testing.T, example string, w, h int) Model {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", example, "plan.json"))
	if err != nil {
		t.Skipf("example %s not generated: %v", example, err)
	}
	g, err := tfplan.Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	g.PlanPath = example + "/plan.json"
	m := New(g)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return nm.(Model)
}

func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(m Model, keys ...string) Model {
	for _, k := range keys {
		nm, _ := m.Update(key(k))
		m = nm.(Model)
	}
	return m
}

func TestViewDimensions(t *testing.T) {
	m := loadUI(t, "simple", 120, 40)
	v := m.View()
	lines := strings.Split(v, "\n")
	if len(lines) != 40 {
		t.Errorf("View has %d lines, want 40", len(lines))
	}
	p := plain(v)
	for _, want := range []string{"tfplantui", "Layer 0", "to add", "random_id.seed"} {
		if !strings.Contains(p, want) {
			t.Errorf("View missing %q\n--- view ---\n%s", want, p)
		}
	}
}

func TestNavigateFollowsEdges(t *testing.T) {
	m := loadUI(t, "simple", 120, 40)
	// initial selection: layer 0, first by address = random_id.seed
	if got := m.selectedNode().Block; got != "random_id.seed" {
		t.Fatalf("initial selection = %s, want random_id.seed", got)
	}
	// right should follow the edge to its dependent in layer 1.
	m = send(m, "right")
	if got := m.selectedNode().Block; got != "random_pet.app" {
		t.Errorf("after right, selection = %s, want random_pet.app", got)
	}
	if m.col != 1 {
		t.Errorf("after right, col = %d, want 1", m.col)
	}
	// right again to layer 2 dependent.
	m = send(m, "right")
	if got := m.selectedNode().Block; got != "random_password.db" {
		t.Errorf("after right x2, selection = %s, want random_password.db", got)
	}
	// left should walk back toward a dependency.
	m = send(m, "left")
	if m.col != 1 {
		t.Errorf("after left, col = %d, want 1", m.col)
	}
}

func TestExpandShowsDiff(t *testing.T) {
	m := loadUI(t, "simple", 120, 40)
	m = send(m, "right") // random_pet.app
	if !strings.Contains(plain(m.View()), "space to expand") {
		t.Errorf("expected expand hint before expanding")
	}
	m = send(m, "space")
	p := plain(m.View())
	for _, want := range []string{"length", "(known after apply)", "separator"} {
		if !strings.Contains(p, want) {
			t.Errorf("expanded view missing %q\n--- view ---\n%s", want, p)
		}
	}
	// collapse again
	m = send(m, "space")
	if strings.Contains(plain(m.View()), "(known after apply)") {
		t.Errorf("diff still visible after collapse")
	}
}

func TestListModeAndExpand(t *testing.T) {
	m := loadUI(t, "medium", 120, 40)
	m = send(m, "t") // list mode
	if m.mode != modeList {
		t.Fatalf("expected list mode")
	}
	p := plain(m.View())
	if !strings.Contains(p, "random_string.api_key") {
		t.Errorf("list view missing api_key\n%s", p)
	}
}

func TestSearchJumps(t *testing.T) {
	m := loadUI(t, "simple", 120, 40)
	m = send(m, "/")
	if !m.searching {
		t.Fatalf("expected search mode")
	}
	m = send(m, "token")
	m = send(m, "enter")
	if got := m.selectedNode().Block; got != "random_string.token" {
		t.Errorf("search jumped to %s, want random_string.token", got)
	}
}

func TestHideNoOpFilter(t *testing.T) {
	m := loadUI(t, "medium", 120, 40)
	before := len(m.listItems())
	m = send(m, "n") // changes-only
	after := len(m.listItems())
	if after >= before {
		t.Errorf("hideNoOp did not reduce items: %d -> %d", before, after)
	}
	// selection must remain visible.
	if !m.nodeVisible(m.selectedNode()) {
		t.Errorf("selection is hidden after enabling changes-only filter")
	}
	for _, n := range m.listItems() {
		if n.ActionAgg() == tfplan.ActionNoOp {
			t.Errorf("no-op node %s still visible under changes-only", n.Block)
		}
	}
}

func TestColorsEmitted(t *testing.T) {
	// Force truecolor so View() emits SGR codes we can assert on (tests have no TTY).
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// medium has no-op (seed, selected by default), create, replace and delete,
	// and seed has dependents so a dependency arrow is drawn.
	v := loadUI(t, "medium", 150, 35).View()

	want := map[string]string{
		"selection background #161B22": "48;2;22;27;34",
		"selection white text #FFFFFF": "38;2;255;255;255",
		"dependency arrow #768390":     "38;2;118;131;144",
		"create green #3FB950":         "38;2;63;185;80",
		"replace purple #BC8CFF":       "38;2;188;140;255",
		"delete red #F85149":           "38;2;248;81;73",
		"mixed yellow #F2CC60":         "38;2;242;204;96",
		"no-op dim #414850":            "38;2;65;72;80",
	}
	for name, code := range want {
		if !strings.Contains(v, code) {
			t.Errorf("missing %s (%q) in rendered output", name, code)
		}
	}
	// Retired colors must be gone: gold selection, teal link, orange mixed, and
	// the grey selection border (#30363D) — selected borders keep their action hue.
	for _, gone := range []string{"227;179;65" /*gold*/, "45;212;191" /*teal*/, "219;109;40" /*orange*/, "48;54;61" /*sel border grey*/} {
		if strings.Contains(v, gone) {
			t.Errorf("retired color %q still present", gone)
		}
	}
}

func TestDependencyArrows(t *testing.T) {
	// Examine the graph body only; the header legend also contains a "─▶" sample.
	body := func(m Model) string {
		lines := strings.Split(plain(m.View()), "\n")
		if len(lines) > headerHeight {
			lines = lines[headerHeight:]
		}
		return strings.Join(lines, "\n")
	}

	// random_id.seed (default selection) has a dependent (random_pet.app), so a
	// just-in-time arrow should be drawn out of it.
	m := loadUI(t, "simple", 120, 40)
	if got := m.selectedNode().Block; got != "random_id.seed" {
		t.Fatalf("expected random_id.seed selected, got %s", got)
	}
	if !strings.Contains(body(m), "▶") {
		t.Errorf("expected a dependency arrow when a node with dependents is selected")
	}

	// random_uuid.request_id has no dependencies or dependents -> no arrows.
	m2 := send(loadUI(t, "simple", 120, 40), "down", "down")
	if got := m2.selectedNode().Block; got != "random_uuid.request_id" {
		t.Fatalf("expected random_uuid.request_id, got %s", got)
	}
	if strings.Contains(body(m2), "▶") {
		t.Errorf("an isolated node should draw no arrows")
	}
}

func TestListFilterNoPanic(t *testing.T) {
	// Regression: filtering the list (n) after jumping to the bottom (G) used to
	// leave listSel out of range and panic in renderList on the next View().
	m := loadUI(t, "medium", 120, 40)
	m = send(m, "t", "G", "n")
	_ = m.View() // must not panic
	if m.selectedNode() == nil {
		t.Errorf("selection lost after filtering")
	}
	// toggle back and forth a few more times
	m = send(m, "n", "G", "n", "G")
	_ = m.View()
}

func TestLargeRenders(t *testing.T) {
	// Only runs if the large example exists; guards against panics/scaling bugs.
	m := loadUI(t, "large", 160, 50)
	v := m.View()
	if len(strings.Split(v, "\n")) != 50 {
		t.Errorf("large view wrong height")
	}
	// exercise some navigation without panicking
	m = send(m, "right", "right", "down", "down", "space", "t", "G", "space")
	_ = m.View()
}
