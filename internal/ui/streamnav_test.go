package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

func TestStreamTogglePreservesCompactSelectionAndPreview(t *testing.T) {
	m := send(loadUI(t, "medium", 120, 40), "right", "tab", "tab", "space")
	selected, instance, original := m.selectedNode(), m.shownInstance(m.selectedNode()), m.View()
	m = send(m, "a")
	if !m.showStreams || m.selectedNode() != selected || m.shownInstance(selected) != instance {
		t.Fatal("enabling streams lost the selected resource instance")
	}
	total := 0
	for c := range m.g.Layers {
		total += len(m.streamColumnCards(c))
	}
	if total != m.g.TotalInstances() {
		t.Fatalf("stream graph exposes %d instances, want %d", total, m.g.TotalInstances())
	}
	m = send(m, "a")
	if m.showStreams || m.View() != original {
		t.Fatal("disabling streams did not restore the compact graph and preview")
	}
}

func TestStreamPreviewStaysOnItsInstance(t *testing.T) {
	m := detailModel(
		&tfplan.Resource{Block: "test.app", Index: "0", Action: tfplan.ActionCreate, After: map[string]any{"name": "only-first"}},
		&tfplan.Resource{Block: "test.app", Index: "1", Action: tfplan.ActionCreate, After: map[string]any{"name": "only-second"}},
	)
	m = send(m, "a", "space", "tab")
	view := plain(m.View())
	if !strings.Contains(view, "only-first") || strings.Contains(view, "only-second") {
		t.Fatalf("preview expansion leaked across instances:\n%s", view)
	}
	m = send(m, "space")
	if !strings.Contains(plain(m.View()), "only-second") {
		t.Fatal("the second instance could not be expanded independently")
	}
	m = send(m, "a")
	if strings.Contains(plain(m.View()), "only-second") {
		t.Fatal("instance previews overwrote the compact block's expansion state")
	}
}

func TestStreamNavigationSkipsUnchangedInstances(t *testing.T) {
	m := detailModel(
		&tfplan.Resource{Block: "test.app", Index: "0", Action: tfplan.ActionNoOp},
		&tfplan.Resource{Block: "test.app", Index: "1", Action: tfplan.ActionCreate},
		&tfplan.Resource{Block: "test.app", Index: "2", Action: tfplan.ActionDelete},
	)
	m = send(m, "a", "n")
	for _, step := range []struct {
		key   string
		index int
	}{{"g", 1}, {"down", 2}, {"up", 1}, {"shift+tab", 2}, {"tab", 1}, {"G", 2}} {
		m = send(m, step.key)
		if got := m.shownInstance(m.selectedNode()); got != step.index {
			t.Fatalf("%s selected instance %d, want %d", step.key, got, step.index)
		}
		if strings.Contains(plain(m.View()), "test.app[0]") {
			t.Fatalf("%s exposed a hidden unchanged instance", step.key)
		}
	}
}

func TestStreamSearchFindsAnIndexedAddress(t *testing.T) {
	m := send(loadUI(t, "medium", 120, 30), "a", "/", "random_integer.port[3]", "enter")
	if m.selectedNode().Block != "random_integer.port" || m.shownInstance(m.selectedNode()) != 3 {
		t.Fatalf("indexed search selected %s instance %d", m.selectedNode().Block, m.shownInstance(m.selectedNode()))
	}
	if !strings.Contains(plain(m.View()), "random_integer.port[3]") {
		t.Fatal("the searched instance is not visible")
	}
	m = send(m, "a")
	if m.shownInstance(m.selectedNode()) != 3 {
		t.Fatal("collapsing streams changed the searched instance")
	}
}

func TestStreamRelationsUseReferencesNotSelectedIndex(t *testing.T) {
	m := send(loadUI(t, "medium", 120, 30), "right", "a", "tab", "tab")
	port := m.selectedNode()
	confirmed, _ := m.streamRelated(port, m.shownInstance(port), false)
	if confirmed["random_pet.release"] {
		t.Fatal("release was linked to port[2], although it references only port[0]")
	}
	if strings.Contains(plain(m.renderContext()), "random_pet.release") {
		t.Fatal("the footer claimed port[2] is used by a resource that references only port[0]")
	}
	m = send(m, "shift+tab", "shift+tab")
	confirmed, _ = m.streamRelated(port, m.shownInstance(port), false)
	if !confirmed["random_pet.release"] {
		t.Fatal("release's confirmed port[0] dependency was lost")
	}
	if !strings.Contains(plain(m.renderContext()), "random_pet.release") {
		t.Fatal("the footer omitted a confirmed dependent of port[0]")
	}
	m = send(m, "f", "a", "a", "right")
	if m.detailLines == nil || !m.showStreams || m.selectedNode().Block != "random_pet.release" {
		t.Fatal("streams could not be toggled while navigating the enlarged card")
	}
	if !strings.Contains(plain(strings.Join(m.detailLines, "\n")), "random_integer.port[0]") {
		t.Fatal("focused resource did not expose its confirmed upstream address")
	}
}

func TestStreamLabelsFitNarrowAndWideCharacterColumns(t *testing.T) {
	m := detailModel(&tfplan.Resource{Block: "test.app", Index: `"europe-界界界.x"`, Action: tfplan.ActionCreate})
	m = send(m, "a")
	n := m.selectedNode()
	for _, width := range []int{13, cardOuter} {
		content := m.streamCardContent(n, 0, true, width)
		joined := strings.Join(strings.Fields(plain(strings.Join(content, ""))), "")
		if !strings.Contains(joined, n.Instances[0].Address()) {
			t.Fatalf("instance address was truncated at width %d: %s", width, joined)
		}
		height := m.streamCardHeightWidth(n, 0, width)
		box := m.renderStreamColumnWindow(0, n, 0, height, width)
		for _, line := range box {
			if lipgloss.Width(line) > width {
				t.Fatalf("instance card exceeds width %d: %q", width, line)
			}
		}
		if !strings.Contains(box[height-1], "┛") {
			t.Fatalf("measured height clipped the selected card at width %d", width)
		}
	}
}

func TestStreamViewBoundsAcrossModesAndFilters(t *testing.T) {
	for _, width := range []int{32, 80, 120, 160} {
		m := loadUI(t, "medium", width, 20)
		for _, input := range []string{"a", "right", "tab", "n", "G", "f", "a", "a", "left", "f", "t", "a"} {
			m = send(m, input)
			assertDetailFits(t, m)
		}
	}
}
