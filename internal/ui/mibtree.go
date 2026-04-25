package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/amos-mib/amos/internal/mib"
)

// MIBTree is the left-panel collapsible OID hierarchy widget.
type MIBTree struct {
	tree     *widget.Tree
	onSelect func(*mib.Node)
	nodes    map[widget.TreeNodeID]*mib.Node
	children map[widget.TreeNodeID][]widget.TreeNodeID
	search   *widget.Entry
	wrap     *fyne.Container
}

// NewMIBTree creates an empty MIBTree.  onSelect is called when the user clicks a node.
func NewMIBTree(onSelect func(*mib.Node)) *MIBTree {
	m := &MIBTree{
		onSelect: onSelect,
		nodes:    make(map[widget.TreeNodeID]*mib.Node),
		children: make(map[widget.TreeNodeID][]widget.TreeNodeID),
	}

	m.tree = &widget.Tree{
		ChildUIDs: func(uid widget.TreeNodeID) []widget.TreeNodeID {
			return m.children[uid]
		},
		IsBranch: func(uid widget.TreeNodeID) bool {
			return len(m.children[uid]) > 0
		},
		CreateNode: func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("")
		},
		UpdateNode: func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			lbl := obj.(*widget.Label)
			if n, ok := m.nodes[uid]; ok {
				if n.Name != n.NumericOID {
					lbl.SetText(n.Name + " (" + n.NumericOID + ")")
				} else {
					lbl.SetText(n.NumericOID)
				}
			}
		},
		OnSelected: func(uid widget.TreeNodeID) {
			if n, ok := m.nodes[uid]; ok && m.onSelect != nil {
				m.onSelect(n)
			}
		},
	}

	m.search = widget.NewEntry()
	m.search.SetPlaceHolder("Search OID or name…")
	m.search.OnChanged = func(query string) {
		m.filterTree(query)
	}

	m.wrap = container.NewBorder(m.search, nil, nil, nil, m.tree)
	return m
}

// SetTree rebuilds the widget from the given root node.
func (m *MIBTree) SetTree(root *mib.Node) {
	m.nodes = make(map[widget.TreeNodeID]*mib.Node)
	m.children = make(map[widget.TreeNodeID][]widget.TreeNodeID)
	if root != nil {
		m.buildIndex("", root)
	}
	fyne.Do(func() { m.tree.Refresh() })
}

func (m *MIBTree) buildIndex(parentID widget.TreeNodeID, n *mib.Node) {
	id := widget.TreeNodeID(n.NumericOID)
	m.nodes[id] = n
	if parentID != "" {
		m.children[parentID] = append(m.children[parentID], id)
	} else {
		m.children[""] = append(m.children[""], id)
	}
	for _, child := range n.Children {
		m.buildIndex(id, child)
	}
}

// filterTree hides nodes whose name/OID does not match query.
// When query is empty the full tree is restored.
func (m *MIBTree) filterTree(query string) {
	// Re-populate children showing only matches + their ancestors.
	if query == "" {
		// Reload from cached nodes — rebuild the parent→child relationships.
		newChildren := make(map[widget.TreeNodeID][]widget.TreeNodeID)
		for id, node := range m.nodes {
			parentID := parentOf(id)
			newChildren[parentID] = append(newChildren[parentID], widget.TreeNodeID(id))
			_ = node
		}
		m.children = newChildren
	} else {
		visible := make(map[widget.TreeNodeID]bool)
		lowerQ := toLower(query)
		for id, node := range m.nodes {
			if contains(toLower(node.Name), lowerQ) || contains(toLower(node.NumericOID), lowerQ) {
				// Mark this node and all ancestors as visible.
				cur := widget.TreeNodeID(id)
				for cur != "" {
					visible[cur] = true
					cur = parentOf(string(cur))
				}
			}
		}
		newChildren := make(map[widget.TreeNodeID][]widget.TreeNodeID)
		for id := range visible {
			pID := parentOf(string(id))
			if visible[pID] || pID == "" {
				newChildren[pID] = append(newChildren[pID], id)
			}
		}
		m.children = newChildren
	}
	m.tree.Refresh()
}

// Container returns the Fyne container for layout embedding.
func (m *MIBTree) Container() fyne.CanvasObject {
	return m.wrap
}

func parentOf(oid string) widget.TreeNodeID {
	for i := len(oid) - 1; i >= 0; i-- {
		if oid[i] == '.' {
			return widget.TreeNodeID(oid[:i])
		}
	}
	return ""
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
