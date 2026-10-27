package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

func detailModel(resources ...*tfplan.Resource) Model {
	n := &tfplan.Node{Block: resources[0].Block, Instances: resources}
	g := &tfplan.Graph{Nodes: []*tfplan.Node{n}, Layers: [][]*tfplan.Node{{n}}, ByBlock: map[string]*tfplan.Node{n.Block: n}}
	m := New(g)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	return updated.(Model)
}

func detailEvent(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func assertDetailFits(t *testing.T, m Model) {
	t.Helper()
	rows := strings.Split(m.View(), "\n")
	if len(rows) != m.height {
		t.Fatalf("screen height = %d, want %d", len(rows), m.height)
	}
	for _, row := range rows {
		if got := lipgloss.Width(row); got > m.width {
			t.Errorf("row width = %d, limit %d: %s", got, m.width, row)
		}
	}
}

func TestDetailPreservesWrappedValuesAndMasks(t *testing.T) {
	long := strings.Repeat("organization-ipam-界", 12) + "END"
	address := "module.network.module.ipam.aws_vpc_ipam_pool.environment"
	for _, width := range []int{32, 80, 120} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := detailModel(&tfplan.Resource{
				Block: address, Action: tfplan.ActionCreate,
				After: map[string]any{
					"description": long,
					"rules":       []any{map[string]any{"cidr": "10.20.0.0/16", "region": "eu-west-1"}},
					"password":    "hidden-secret",
					"tokens":      []any{"public-token", "hidden-token"},
				},
				AfterUnknown:   map[string]any{"id": true},
				AfterSensitive: map[string]any{"password": true, "tokens": []any{false, true}},
				ReplacePaths:   [][]any{{"rules"}},
			})
			m = detailEvent(m, tea.WindowSizeMsg{Width: width, Height: 20})
			m = send(m, "f")
			content := plain(strings.Join(m.detailLines, "\n"))
			joined := strings.Join(strings.Fields(content), "")
			for _, want := range []string{long, address, "10.20.0.0/16", "eu-west-1", "(knownafterapply)", "(sensitivevalue)", "#forcesreplacement"} {
				if !strings.Contains(joined, want) {
					t.Errorf("detail lost %q:\n%s", want, content)
				}
			}
			for _, secret := range []string{"hidden-secret", "hidden-token", "public-token"} {
				if strings.Contains(joined, secret) {
					t.Errorf("detail leaked masked collection or value: %s", secret)
				}
			}
			assertDetailFits(t, m)
		})
	}
}

func TestDetailScrollResizeAndReturn(t *testing.T) {
	after := map[string]any{"zz_last": "last-value", "unchanged": "existing"}
	for i := range 45 {
		after[fmt.Sprintf("attribute_%02d", i)] = fmt.Sprintf("value-%02d", i)
	}
	m := detailModel(&tfplan.Resource{
		Block: "aws_vpc_ipam.this", Action: tfplan.ActionUpdate,
		Before: map[string]any{"unchanged": "existing"}, After: after,
	})
	m = send(m, "space")
	original := m.View()
	m = send(m, "f", "j")
	if m.detailScroll != 1 {
		t.Fatalf("line scrolling = %d, want 1", m.detailScroll)
	}
	m = detailEvent(m, tea.KeyMsg{Type: tea.KeyPgDown})
	if m.detailScroll != 1+m.detailHeight() {
		t.Fatalf("page scrolling skipped the wrong number of rows: %d", m.detailScroll)
	}
	m = send(m, "G")
	if !strings.Contains(plain(m.View()), "zz_last = \"last-value\"") {
		t.Fatalf("last attribute is unreachable:\n%s", plain(m.View()))
	}
	m = send(m, "?", "j")
	if !m.showHelp {
		t.Fatal("detail help closed on a navigation key")
	}
	scroll := m.detailScroll
	m = detailEvent(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.showHelp || m.detailLines == nil || m.detailScroll != scroll {
		t.Fatal("closing help should preserve detail view and scroll position")
	}
	m = detailEvent(m, tea.WindowSizeMsg{Width: 120, Height: 70})
	if m.detailScroll != 0 || !strings.Contains(plain(m.View()), "last-value") {
		t.Fatal("growing the window should clamp scrolling and expose the content")
	}
	assertDetailFits(t, m)
	m = detailEvent(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = send(m, "u", "G")
	if !strings.Contains(plain(strings.Join(m.detailLines, "\n")), `unchanged = "existing"`) {
		t.Fatal("unchanged attributes not available in detail")
	}
	m = send(m, "u")
	m = detailEvent(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.detailLines != nil || m.View() != original {
		t.Fatal("closing details did not restore the expanded graph and selection")
	}
}

func TestDetailCyclesInstancesFromList(t *testing.T) {
	for _, keys := range [][2]string{{"[", "]"}, {"shift+tab", "tab"}} {
		t.Run(keys[0]+"/"+keys[1], func(t *testing.T) {
			m := detailModel(
				&tfplan.Resource{Block: "aws_ram_resource_share.regional", Index: `"east"`, Action: tfplan.ActionCreate, After: map[string]any{"name": "east-share"}},
				&tfplan.Resource{Block: "aws_ram_resource_share.regional", Index: `"west"`, Action: tfplan.ActionDelete, Before: map[string]any{"name": "west-share"}},
				&tfplan.Resource{Block: "aws_ram_resource_share.regional", Index: `"north"`, Action: tfplan.ActionCreate, After: map[string]any{"name": "north-share"}},
			)
			m = send(m, "t", "f")
			if !strings.Contains(plain(m.View()), `aws_ram_resource_share.regional["east"]`) {
				t.Fatal("full instance address missing")
			}
			m = send(m, keys[1])
			for _, want := range []string{`aws_ram_resource_share.regional["west"]`, "instance 2/3", `- name = "west-share"`} {
				if !strings.Contains(plain(m.View()), want) {
					t.Errorf("selected instance missing %q:\n%s", want, plain(m.View()))
				}
			}
			m = send(m, keys[1], keys[1])
			if !strings.Contains(plain(m.View()), "east-share") {
				t.Fatal("next instance should wrap around")
			}
			m = send(m, keys[0])
			if !strings.Contains(plain(m.View()), "north-share") {
				t.Fatal("previous instance should wrap around to the last instance")
			}
			m = send(m, keys[0], "f")
			if m.mode != modeList || m.detailLines != nil || m.shownInstance(m.selectedNode()) != 1 {
				t.Fatal("return should preserve list selection and last inspected instance")
			}
		})
	}
}

func TestTabCyclesInstancesInGraphAndList(t *testing.T) {
	for _, mode := range []viewMode{modeGraph, modeList} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			m := send(loadUI(t, "medium", 120, 40), "right")
			if mode == modeList {
				m = send(m, "t")
			}
			selected := m.selectedNode()
			m = send(m, "space", "tab")
			if !strings.Contains(plain(m.View()), "instance 2/5") {
				t.Fatalf("Tab should display the next instance:\n%s", plain(m.View()))
			}
			m = send(m, "shift+tab", "shift+tab")
			if !strings.Contains(plain(m.View()), "instance 5/5") {
				t.Fatalf("Shift+Tab should cycle backward and wrap to the last instance:\n%s", plain(m.View()))
			}
			if m.selectedNode() != selected || m.mode != mode {
				t.Fatal("cycling instances should preserve the selected resource and view")
			}
		})
	}
}

func TestDetailNavigatesLevelsWithoutCyclingInstances(t *testing.T) {
	for _, mode := range []viewMode{modeGraph, modeList} {
		for _, keys := range [][2]string{{"left", "right"}, {"h", "l"}} {
			t.Run(fmt.Sprintf("%d-%s", mode, keys[0]), func(t *testing.T) {
				m := send(loadUI(t, "medium", 120, 12), "right", "tab")
				if mode == modeList {
					m = send(m, "t")
				}
				m = send(m, "f", "G", keys[1])
				if m.selectedNode().Block != "random_pet.release" || m.detailLines == nil || m.detailScroll != 0 {
					t.Fatalf("next level should open its resource at the top:\n%s", plain(m.View()))
				}
				if !strings.Contains(plain(m.renderDetail()), "random_pet.release") {
					t.Fatal("focused card still displays the previous resource")
				}
				m = send(m, keys[0])
				if m.selectedNode().Block != "random_integer.port" || m.shownInstance(m.selectedNode()) != 1 {
					t.Fatal("returning to the previous level should preserve its selected instance")
				}
				m = send(m, keys[0], keys[0])
				if m.selectedNode().Block != "random_id.seed" || m.col != 0 || m.detailLines == nil {
					t.Fatal("left navigation should stop at the first visible level with the card still open")
				}
				m = send(m, keys[1], keys[1], keys[1])
				if m.selectedNode().Block != "random_pet.release" || m.col != 2 || m.detailLines == nil {
					t.Fatal("right navigation should stop at the last visible level with the card still open")
				}
				m = send(m, "f")
				if m.mode != mode || m.selectedNode().Block != "random_pet.release" {
					t.Fatal("closing the card should retain the navigated resource in the original view")
				}
			})
		}
	}
}

func TestDetailEmptyFilteredPlan(t *testing.T) {
	m := New(&tfplan.Graph{})
	m = detailEvent(m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = send(m, "f")
	if m.detailLines != nil || !strings.Contains(plain(m.View()), "up to date") {
		t.Fatal("empty plan should remain in the graph")
	}
	m = detailModel(&tfplan.Resource{Block: "test_resource.noop", Action: tfplan.ActionNoOp})
	m = send(m, "n", "f")
	if m.detailLines != nil || !strings.Contains(plain(m.View()), "No changes to show") {
		t.Fatal("hidden resource should not open in details")
	}
}

func TestEnlargedCardKeepsNeighboringLayersVisible(t *testing.T) {
	makeNode := func(name string, layer int) *tfplan.Node {
		block := "test_resource." + name
		return &tfplan.Node{
			Block: block, Type: "test_resource", Name: name, Layer: layer,
			Instances: []*tfplan.Resource{{Block: block, Action: tfplan.ActionCreate, After: map[string]any{"name": name}}},
		}
	}
	upstream, focused, downstream := makeNode("upstream", 0), makeNode("focused", 1), makeNode("downstream", 2)
	focused.DepsOut, focused.DepsIn = []string{upstream.Block}, []string{downstream.Block}
	g := &tfplan.Graph{
		Nodes:   []*tfplan.Node{upstream, focused, downstream},
		Layers:  [][]*tfplan.Node{{upstream}, {focused}, {downstream}},
		ByBlock: map[string]*tfplan.Node{upstream.Block: upstream, focused.Block: focused, downstream.Block: downstream},
	}
	for _, width := range []int{80, 120, 180} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := detailEvent(New(g), tea.WindowSizeMsg{Width: width, Height: 30})
			m = send(m, "right", "f")
			assertDetailFits(t, m)
			body := plain(m.renderDetail())
			rows := strings.Split(body, "\n")
			// The expanded border stays centered and wider than an ordinary card.
			borderY, borderX, borderWidth := -1, -1, 0
			for y, row := range rows {
				start, end := strings.IndexRune(row, '┏'), strings.IndexRune(row, '┓')
				if start >= 0 && end > start {
					borderY, borderX = y, lipgloss.Width(row[:start])
					borderWidth = lipgloss.Width(row[start:end]) + 1
					break
				}
			}
			if borderY < 2 || borderWidth <= cardOuter || abs(borderX-(width-borderX-borderWidth)) > 1 {
				t.Fatalf("expanded card is not larger and centered: x=%d y=%d width=%d\n%s", borderX, borderY, borderWidth, body)
			}
			neighborsVisible := false
			for _, row := range rows {
				if borderX > 0 && strings.Contains(string([]rune(row)[:borderX]), "test_") {
					neighborsVisible = strings.Contains(string([]rune(row)[borderX+borderWidth:]), "test_")
					break
				}
			}
			if !neighborsVisible {
				t.Fatalf("neighboring resources should flank the focused card:\n%s", body)
			}
			if !strings.Contains(body, "▶") {
				t.Fatalf("dependency connectors disappeared:\n%s", body)
			}
			m = send(m, "f")
			if m.selectedNode() != focused || m.col != 1 {
				t.Fatal("collapsing should preserve the original graph selection")
			}
		})
	}
}
