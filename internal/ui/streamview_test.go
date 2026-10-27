package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

func TestStreamCardsFilterInstancesAndKeepExpansionIdentity(t *testing.T) {
	m := detailModel(
		&tfplan.Resource{Block: "test.app", Index: "0", Action: tfplan.ActionNoOp},
		&tfplan.Resource{Block: "test.app", Index: "1", Action: tfplan.ActionCreate},
		&tfplan.Resource{Block: "test.app", Index: "2", Action: tfplan.ActionDelete},
	)
	m.showStreams = true
	m.hideNoOp = true
	n := m.g.At(0, 0)
	m.instanceIdx[n.Block] = 2
	cards := m.streamColumnCards(0)
	if len(cards) != 2 || cards[0].idx != 1 || cards[1].idx != 2 {
		t.Fatalf("filtered stream cards = %+v", cards)
	}
	if m.cardHeight(n) != cards[0].height+cards[1].height {
		t.Fatal("block height does not include every visible instance")
	}
	centers := m.streamInstanceCenters(0, 0)
	if _, ok := centers["test.app[0]"]; ok {
		t.Fatal("hidden unchanged instance has an anchor")
	}
	if m.columnCenters(0, 0)[0] != centers["test.app[2]"] {
		t.Fatal("block anchor does not follow selected instance")
	}
	content := plain(strings.Join(m.renderColumnWindow(0, n, 0, m.cardHeight(n), cardOuter), "\n"))
	if strings.Contains(content, "test.app[0]") || !strings.Contains(content, "test.app[1]") || !strings.Contains(content, "test.app[2]") {
		t.Fatalf("incorrect stream addresses:\n%s", content)
	}
}

func TestStreamReferenceAnnotationsPreserveScope(t *testing.T) {
	m := detailModel(&tfplan.Resource{Block: "test.release", Index: "1", Action: tfplan.ActionCreate})
	m.showStreams = true
	n := m.g.At(0, 0)
	n.InstanceDeps = []tfplan.InstanceDependency{
		{Block: "test.port", Instances: []string{"test.port[0]"}},
		{Block: "test.service", Unresolved: true},
	}
	content := strings.Join(strings.Fields(plain(strings.Join(m.streamCardContent(n, 0, false, cardOuter), "\n"))), "")
	for _, want := range []string{"test.release[1]", "confirmedupstream:test.port[0]", "?blockscope:test.service"} {
		if !strings.Contains(content, want) {
			t.Errorf("missing scoped reference %q in %s", want, content)
		}
	}
	if strings.Contains(content, "test.port[1]") || strings.Contains(content, "test.service[1]") {
		t.Fatal("stream rendering invented same-index references")
	}
	box := m.renderStreamColumnWindow(0, n, 0, m.streamCardHeight(n, 0), cardOuter)
	if len(box) != m.streamCardHeight(n, 0) {
		t.Fatal("window height changed")
	}
	for _, line := range box {
		if lipgloss.Width(line) > cardOuter {
			t.Fatalf("stream card exceeds column width: %q", line)
		}
	}
	// An unresolved dependency alone must not create any instance connector.
	left := []streamCard{{node: &tfplan.Node{Instances: []*tfplan.Resource{{Block: "test.service", Index: "1"}}}, height: 3}}
	right := []streamCard{{node: n, height: m.streamCardHeight(n, 0)}}
	if m.streamGap(left, right, 0, 40) != nil {
		t.Fatal("block-scoped dependency was expanded into an instance edge")
	}
}
