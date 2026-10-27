package ui

import (
	"sort"
	"strconv"
	"strings"

	"github.com/omarismail/terraform-plan-tui/internal/tfplan"
)

func (m *Model) toggleStreams() {
	focused := m.detailLines != nil
	if m.mode == modeList && !focused {
		m.syncGraphToList()
		m.mode = modeGraph
	}
	m.showStreams = !m.showStreams
	m.snapSelection()
	m.ensureColVisible()
	if focused {
		m.detailScroll = 0
		m.rebuildDetail()
	}
}

func (m *Model) selectStreamEnd(last bool) {
	if !m.showStreams {
		return
	}
	n := m.g.At(m.col, m.row)
	if n == nil {
		return
	}
	instances := m.visibleInstanceIndices(n)
	if len(instances) == 0 {
		return
	}
	idx := instances[0]
	if last {
		idx = instances[len(instances)-1]
	}
	m.instanceIdx[n.Block] = idx
}

// Search the actual displayed addresses without constructing another graph.
// A wrapped match is saved while scanning for the first match after selection.
func (m Model) searchStream(query string) (col, row, instance int, ok bool) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return 0, 0, 0, false
	}
	pastSelection := false
	selected := m.g.At(m.col, m.row)
	selectedInstance := -1
	if selected != nil {
		selectedInstance = m.shownInstance(selected)
	}
	for c := range m.g.Layers {
		for _, r := range m.visibleRows(c) {
			n := m.g.At(c, r)
			for _, idx := range m.visibleInstanceIndices(n) {
				matches := strings.Contains(strings.ToLower(n.Instances[idx].Address()), query)
				if matches && !ok {
					col, row, instance, ok = c, r, idx, true
				}
				if c == m.col && r == m.row && idx == selectedInstance {
					pastSelection = true
					continue
				}
				if pastSelection && matches {
					return c, r, idx, true
				}
			}
		}
	}
	return
}

func (m Model) streamRelated(n *tfplan.Node, idx int, upstream bool) (map[string]bool, map[string]bool) {
	confirmed, grouped := map[string]bool{}, map[string]bool{}
	if upstream {
		for _, dep := range n.InstanceDeps {
			for _, address := range dep.Instances {
				if m.streamAddressVisible(dep.Block, address) {
					confirmed[address] = true
				}
			}
			if dep.Unresolved {
				grouped[dep.Block] = true
			}
		}
		return confirmed, grouped
	}
	address := n.Instances[idx].Address()
	for _, block := range n.DepsIn {
		child := m.g.ByBlock[block]
		if !m.nodeVisible(child) {
			continue
		}
		for _, dep := range child.InstanceDeps {
			if dep.Block != n.Block {
				continue
			}
			if dep.Unresolved {
				grouped[child.Block] = true
			}
			for _, target := range dep.Instances {
				if target == address {
					for _, i := range m.visibleInstanceIndices(child) {
						confirmed[child.Instances[i].Address()] = true
					}
					break
				}
			}
		}
	}
	return confirmed, grouped
}

func (m Model) renderStreamDetailNeighbor(col, width, height, centerY int, confirmed, grouped map[string]bool) ([]string, []int) {
	cards := m.streamColumnCardsWidth(col, width)
	scroll := 0
	if len(cards) > 0 {
		target := cards[0]
		for _, card := range cards {
			if grouped[card.node.Block] {
				target = card
				break
			}
		}
		for _, card := range cards {
			if confirmed[card.node.Instances[card.idx].Address()] {
				target = card
				break
			}
		}
		scroll = target.y + target.height/2 - centerY
	}
	var connected []int
	for _, card := range cards {
		y := card.y - scroll + card.height/2
		if y >= 0 && y < height && confirmed[card.node.Instances[card.idx].Address()] {
			connected = append(connected, y)
		}
	}
	return m.renderColumnWindow(col, nil, scroll, height, width), connected
}

func (m Model) streamContextLinks(title string, confirmed, grouped map[string]bool) string {
	labels := make([]string, 0, len(confirmed)+len(grouped))
	for address := range confirmed {
		labels = append(labels, address)
	}
	for block := range grouped {
		labels = append(labels, block+" (block scope)")
	}
	if len(labels) == 0 {
		return ""
	}
	sort.Strings(labels)
	text := strings.Join(labels[:min(3, len(labels))], ", ")
	if len(labels) > 3 {
		text += " +" + strconv.Itoa(len(labels)-3)
	}
	return m.styles.faint.Render("   "+title+" ") + m.styles.dim.Render(text)
}
