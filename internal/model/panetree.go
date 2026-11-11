// Package model holds Code's workspace, tab and split-pane structure, the
// session it saves, and nothing about terminals or widgets, so it can be
// tested without a window.
package model

// Axis is the direction a split divides its two panes.
type Axis string

const (
	// Horizontal puts the panes side by side.
	Horizontal Axis = "horizontal"
	// Vertical stacks the panes.
	Vertical Axis = "vertical"
)

// Direction names where a new pane goes relative to an existing one.
type Direction int

const (
	DirRight Direction = iota
	DirLeft
	DirDown
	DirUp
)

// Navigation names a pane to move to.
type Navigation int

const (
	NavPrevious Navigation = iota
	NavNext
	NavUp
	NavLeft
	NavDown
	NavRight
)

// Node is one pane in a tab's split layout: either a single leaf, or two
// children divided along an axis.
type Node[L comparable] struct {
	// Leaf is the pane's terminal; only meaningful when HasLeaf is true.
	Leaf L
	// HasLeaf distinguishes a leaf node from a split.
	HasLeaf bool
	Axis    Axis
	First   *Node[L]
	Second  *Node[L]
	// Pct is the share of the split the first pane takes, 0 to 100.
	Pct    float32
	Parent *Node[L]
}

// NewLeaf makes a node holding one pane.
func NewLeaf[L comparable](leaf L) *Node[L] {
	return &Node[L]{Leaf: leaf, HasLeaf: true, Pct: 50}
}

// NewSplit makes a node dividing two panes.
func NewSplit[L comparable](axis Axis, first, second *Node[L], pct float32) *Node[L] {
	n := &Node[L]{Axis: axis, First: first, Second: second, Pct: pct}
	if n.Pct <= 0 || n.Pct >= 100 {
		n.Pct = 50
	}
	first.Parent = n
	second.Parent = n
	return n
}

// Leaves returns every pane under the node, in order.
func (n *Node[L]) Leaves() []L {
	var out []L
	n.appendLeaves(&out)
	return out
}

func (n *Node[L]) appendLeaves(out *[]L) {
	if n.HasLeaf {
		*out = append(*out, n.Leaf)
		return
	}
	n.First.appendLeaves(out)
	n.Second.appendLeaves(out)
}

// NodeFor returns the node holding leaf, or nil.
func (n *Node[L]) NodeFor(leaf L) *Node[L] {
	if n.HasLeaf {
		if n.Leaf == leaf {
			return n
		}
		return nil
	}
	if found := n.First.NodeFor(leaf); found != nil {
		return found
	}
	return n.Second.NodeFor(leaf)
}

// Tree is the split layout of one tab. Version changes on every structural
// edit so views know when to rebuild.
type Tree[L comparable] struct {
	Root    *Node[L]
	Version int
	// zoomed is the pane filling the whole tab while the others are hidden.
	zoomed    L
	hasZoomed bool
}

// NewTree makes a one-pane tree.
func NewTree[L comparable](leaf L) *Tree[L] {
	return &Tree[L]{Root: NewLeaf(leaf)}
}

// NewTreeFrom makes a tree around an existing root node.
func NewTreeFrom[L comparable](root *Node[L]) *Tree[L] {
	return &Tree[L]{Root: root}
}

// Leaves returns the tab's panes.
func (t *Tree[L]) Leaves() []L { return t.Root.Leaves() }

// Contains reports whether leaf is in the tree.
func (t *Tree[L]) Contains(leaf L) bool { return t.Root.NodeFor(leaf) != nil }

// NodeFor returns the node holding leaf.
func (t *Tree[L]) NodeFor(leaf L) *Node[L] { return t.Root.NodeFor(leaf) }

// Zoomed returns the zoomed pane, if any.
func (t *Tree[L]) Zoomed() (L, bool) { return t.zoomed, t.hasZoomed }

// ZoomedIndex returns the zoomed pane's index among the leaves, or -1.
func (t *Tree[L]) ZoomedIndex() int {
	if !t.hasZoomed {
		return -1
	}
	for i, leaf := range t.Leaves() {
		if leaf == t.zoomed {
			return i
		}
	}
	return -1
}

// Split puts newLeaf next to leaf on the given side.
func (t *Tree[L]) Split(leaf L, dir Direction, newLeaf L) {
	node := t.Root.NodeFor(leaf)
	if node == nil {
		return
	}
	existing := NewLeaf(leaf)
	added := NewLeaf(newLeaf)
	node.Leaf = zero[L]()
	node.HasLeaf = false
	node.Pct = 50
	switch dir {
	case DirRight:
		node.Axis, node.First, node.Second = Horizontal, existing, added
	case DirLeft:
		node.Axis, node.First, node.Second = Horizontal, added, existing
	case DirDown:
		node.Axis, node.First, node.Second = Vertical, existing, added
	case DirUp:
		node.Axis, node.First, node.Second = Vertical, added, existing
	}
	existing.Parent = node
	added.Parent = node
	t.hasZoomed = false
	t.Version++
}

// Remove drops a pane from the layout, returning the pane that should take
// focus next, and whether one was removed.
func (t *Tree[L]) Remove(leaf L) (L, bool) {
	node := t.Root.NodeFor(leaf)
	if node == nil || node.Parent == nil {
		return zero[L](), false
	}
	parent := node.Parent
	sibling := parent.Second
	if node == parent.Second {
		sibling = parent.First
	}
	parent.Leaf = sibling.Leaf
	parent.HasLeaf = sibling.HasLeaf
	parent.Axis = sibling.Axis
	parent.First = sibling.First
	parent.Second = sibling.Second
	parent.Pct = sibling.Pct
	if parent.First != nil {
		parent.First.Parent = parent
	}
	if parent.Second != nil {
		parent.Second.Parent = parent
	}
	t.hasZoomed = false
	t.Version++
	leaves := parent.Leaves()
	if len(leaves) == 0 {
		return zero[L](), false
	}
	return leaves[0], true
}

// Move puts leaf next to target on the given side.
func (t *Tree[L]) Move(leaf, target L, dir Direction) {
	if leaf == target || !t.Contains(leaf) || !t.Contains(target) {
		return
	}
	if _, ok := t.Remove(leaf); !ok {
		return
	}
	t.Split(target, dir, leaf)
}

// Swap exchanges two panes' positions, keeping the layout and ratios.
func (t *Tree[L]) Swap(a, b L) {
	if a == b {
		return
	}
	an := t.Root.NodeFor(a)
	bn := t.Root.NodeFor(b)
	if an == nil || bn == nil {
		return
	}
	an.Leaf = b
	bn.Leaf = a
	t.hasZoomed = false
	t.Version++
}

// ToggleZoom zooms a pane, or unzooms it when it is already zoomed.
func (t *Tree[L]) ToggleZoom(leaf L) {
	if t.hasZoomed && t.zoomed == leaf {
		t.hasZoomed = false
		t.Version++
		return
	}
	if t.Root.HasLeaf || !t.Contains(leaf) {
		return
	}
	t.zoomed = leaf
	t.hasZoomed = true
	t.Version++
}

// Unzoom shows every pane again.
func (t *Tree[L]) Unzoom() {
	if !t.hasZoomed {
		return
	}
	t.hasZoomed = false
	t.Version++
}

// Equalize resets every split to an even divide.
func (t *Tree[L]) Equalize() {
	var visit func(*Node[L])
	visit = func(n *Node[L]) {
		n.Pct = 50
		if !n.HasLeaf {
			visit(n.First)
			visit(n.Second)
		}
	}
	visit(t.Root)
	t.hasZoomed = false
	t.Version++
}

// DividerExtent returns the share of the whole tree the nearest split along
// the axis dir moves occupies, or 0 when there is none. It is what turns a
// resize in pixels into the percentage of that split's box.
func (t *Tree[L]) DividerExtent(leaf L, dir Direction) float32 {
	node := t.Root.NodeFor(leaf)
	if node == nil {
		return 0
	}
	axis := axisFor(dir)
	for n := node.Parent; n != nil; n = n.Parent {
		if n.Axis == axis {
			return t.nodeExtent(n, axis)
		}
	}
	return 0
}

// Resize moves the divider of the nearest split along the axis dir moves by
// pct of that split's box, positive moving right or down, so that the pane on
// the side the divider leaves grows, as Ghostty's resize_split does. The
// split keeps a visible share of both panes.
func (t *Tree[L]) Resize(leaf L, dir Direction, pct float32) {
	if pct == 0 {
		return
	}
	node := t.Root.NodeFor(leaf)
	if node == nil {
		return
	}
	axis := axisFor(dir)
	split := node.Parent
	for split != nil && split.Axis != axis {
		split = split.Parent
	}
	if split == nil {
		return
	}
	if dir == DirLeft || dir == DirUp {
		pct = -pct
	}
	// Keep both panes visible, so a repeated resize cannot collapse one.
	split.Pct = minF(maxF(split.Pct+pct, 2), 98)
	t.Version++
}

func axisFor(dir Direction) Axis {
	if dir == DirUp || dir == DirDown {
		return Vertical
	}
	return Horizontal
}

// nodeExtent returns n's extent along axis as a share of the whole tree: the
// product of the shares of its ancestors that split along that axis.
func (t *Tree[L]) nodeExtent(target *Node[L], axis Axis) float32 {
	var walk func(*Node[L], float32) (float32, bool)
	walk = func(n *Node[L], extent float32) (float32, bool) {
		if n == target {
			return extent, true
		}
		if n.HasLeaf {
			return 0, false
		}
		first, second := extent, extent
		if n.Axis == axis {
			share := n.Pct / 100
			first, second = extent*share, extent*(1-share)
		}
		if v, ok := walk(n.First, first); ok {
			return v, true
		}
		return walk(n.Second, second)
	}
	extent, _ := walk(t.Root, 1)
	return extent
}

// Neighbor returns the pane to move to in the given direction. The
// directional cases use each pane's share of a unit square, which does not
// depend on the tab's actual size.
func (t *Tree[L]) Neighbor(leaf L, nav Navigation) (L, bool) {
	all := t.Leaves()
	if len(all) < 2 {
		return zero[L](), false
	}
	index := -1
	for i, l := range all {
		if l == leaf {
			index = i
			break
		}
	}
	if index < 0 {
		return zero[L](), false
	}
	switch nav {
	case NavPrevious:
		return all[(index+len(all)-1)%len(all)], true
	case NavNext:
		return all[(index+1)%len(all)], true
	}
	frames := make(map[L]rect, len(all))
	t.collectFrames(t.Root, rect{0, 0, 1, 1}, frames)
	origin, ok := frames[leaf]
	if !ok {
		return zero[L](), false
	}
	best, found := zero[L](), false
	bestDist, bestOverlap := float32(1e9), float32(-1e9)
	for _, other := range all {
		if other == leaf {
			continue
		}
		c := frames[other]
		var dist, overlap float32
		switch nav {
		case NavRight:
			dist = c.x - (origin.x + origin.w)
			overlap = minF(origin.y+origin.h, c.y+c.h) - maxF(origin.y, c.y)
		case NavLeft:
			dist = origin.x - (c.x + c.w)
			overlap = minF(origin.y+origin.h, c.y+c.h) - maxF(origin.y, c.y)
		case NavUp:
			dist = c.y - (origin.y + origin.h)
			overlap = minF(origin.x+origin.w, c.x+c.w) - maxF(origin.x, c.x)
		case NavDown:
			dist = origin.y - (c.y + c.h)
			overlap = minF(origin.x+origin.w, c.x+c.w) - maxF(origin.x, c.x)
		}
		if dist < -0.001 || overlap <= 0 {
			continue
		}
		if dist < bestDist || (dist == bestDist && overlap > bestOverlap) {
			best, found, bestDist, bestOverlap = other, true, dist, overlap
		}
	}
	return best, found
}

type rect struct{ x, y, w, h float32 }

func (t *Tree[L]) collectFrames(n *Node[L], r rect, out map[L]rect) {
	if n.HasLeaf {
		out[n.Leaf] = r
		return
	}
	share := n.Pct / 100
	if n.Axis == Horizontal {
		t.collectFrames(n.First, rect{r.x, r.y, r.w * share, r.h}, out)
		t.collectFrames(n.Second, rect{r.x + r.w*share, r.y, r.w * (1 - share), r.h}, out)
	} else {
		t.collectFrames(n.First, rect{r.x, r.y, r.w, r.h * share}, out)
		t.collectFrames(n.Second, rect{r.x, r.y + r.h*share, r.w, r.h * (1 - share)}, out)
	}
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// zero returns the zero value of L.
func zero[L any]() L {
	var v L
	return v
}
