package editor

import (
	"cmp"
	"math"
	"slices"

	"github.com/phucngodev/code/internal/board/doc"
	"github.com/phucngodev/code/internal/board/geo"
)

// A mindmap is built from ordinary tldraw records: each node is a box, note or
// text shape, and each branch is an arrow whose start binds to the parent and
// whose end binds to the child, so the edges follow the nodes and the drawing
// stays portable. Nothing else marks a node, so any shape can join a mindmap.

const (
	mindNodeW = 180.0 // a new node's width
	mindNodeH = 64.0  // a new node's height
	mindGapX  = 90.0  // room between a node and its children
	mindGapY  = 44.0  // room between sibling subtrees
)

// MindmapNode reports whether a shape can be a mindmap node.
func MindmapNode(s *doc.Shape) bool { return mindNode(s) }

// mindNode reports whether a shape can be a mindmap node.
func mindNode(s *doc.Shape) bool {
	switch s.Type {
	case doc.TypeGeo, doc.TypeNote, doc.TypeText, doc.TypeImage:
		return true
	}
	return false
}

// MindmapChildren returns a node's children, in the order of the arrows that
// bind to it.
func (e *Editor) MindmapChildren(id doc.ID) []doc.ID {
	e.indexBindings()
	var out []doc.ID
	for _, b := range e.bindingsTo(id) {
		if b.Terminal != "start" {
			continue
		}
		eb := e.bindingOf(b.From, "end")
		// An arrow bound to one shape at both ends is that shape's text-leaf
		// underline, not a branch.
		if eb == nil || eb.To == id {
			continue
		}
		if c := e.Doc.Shapes[eb.To]; c != nil && mindNode(c) {
			out = append(out, eb.To)
		}
	}
	// Siblings keep their z-order, so the tree lays out the same every frame.
	slices.SortFunc(out, func(a, b doc.ID) int {
		return cmp.Compare(e.Doc.Shapes[a].Index, e.Doc.Shapes[b].Index)
	})
	return out
}

// MindmapParent returns a node's parent, or "" at a root.
func (e *Editor) MindmapParent(id doc.ID) doc.ID {
	e.indexBindings()
	for _, b := range e.bindingsTo(id) {
		if b.Terminal != "end" {
			continue
		}
		sb := e.bindingOf(b.From, "start")
		// An underline binds to its shape at both ends, so it is no parent.
		if sb == nil || sb.To == id {
			continue
		}
		if p := e.Doc.Shapes[sb.To]; p != nil && mindNode(p) {
			return sb.To
		}
	}
	return ""
}

// MindmapRoot returns the root of the tree a node belongs to.
func (e *Editor) MindmapRoot(id doc.ID) doc.ID {
	seen := map[doc.ID]bool{}
	for !seen[id] {
		seen[id] = true
		p := e.MindmapParent(id)
		if p == "" {
			break
		}
		id = p
	}
	return id
}

// CreateMindmapRoot makes a root node centered at a page point and starts
// editing its label.
func (e *Editor) CreateMindmapRoot(at geo.Vec) doc.ID {
	e.begin()
	id := e.createMindNode(geo.V(at.X-mindNodeW/2, at.Y-mindNodeH/2))
	e.Select(id)
	e.StartEditing(id)
	e.commit()
	return id
}

// AddMindmapChild adds a child under parent, draws the branch and lays the
// tree out, then edits the child's label.
func (e *Editor) AddMindmapChild(parent doc.ID) doc.ID {
	if e.Doc.Shapes[parent] == nil {
		return ""
	}
	root := e.MindmapRoot(parent)
	c := e.nodeCenter(parent)
	e.begin()
	id := e.createMindLeaf(geo.V(c.X-mindNodeW/2, c.Y-mindNodeH/2), e.mindLevel(parent)+1)
	e.connect(parent, id)
	e.layoutTree(root)
	e.Select(id)
	e.StartEditing(id)
	e.commit()
	return id
}

// AddMindmapSibling adds a node beside node, under the same parent; a root has
// no sibling, so it gains a child instead.
func (e *Editor) AddMindmapSibling(node doc.ID) doc.ID {
	if p := e.MindmapParent(node); p != "" {
		return e.AddMindmapChild(p)
	}
	return e.AddMindmapChild(node)
}

// PromoteMindmapNode moves a node up to its grandparent, or makes it a root.
func (e *Editor) PromoteMindmapNode(node doc.ID) {
	p := e.MindmapParent(node)
	if p == "" {
		return
	}
	gp := e.MindmapParent(p)
	e.begin()
	e.removeEdge(p, node)
	root := node
	if gp != "" {
		e.connect(gp, node)
		root = e.MindmapRoot(node)
	}
	e.layoutTree(root)
	e.commit()
}

// ArrangeAsMindmap lays the selected shapes out as a mindmap. The arrows
// between them are the tree; when they have none, the first selected shape
// becomes the root and the rest its children.
func (e *Editor) ArrangeAsMindmap() {
	var nodes []doc.ID
	for _, id := range e.Selection() {
		if s := e.Doc.Shapes[id]; s != nil && mindNode(s) {
			nodes = append(nodes, id)
		}
	}
	if len(nodes) == 0 {
		return
	}
	e.begin()
	if !e.edgeAmong(nodes) {
		for _, n := range nodes[1:] {
			e.connect(nodes[0], n)
		}
	}
	var roots []doc.ID
	for _, n := range nodes {
		if p := e.MindmapParent(n); p == "" || !slices.Contains(nodes, p) {
			roots = append(roots, n)
		}
	}
	y := math.Inf(1)
	for _, n := range nodes {
		y = math.Min(y, e.nodeCenter(n).Y)
	}
	for _, r := range roots {
		h := e.treeHeight(r)
		e.layoutTreeAt(r, e.nodeCenter(r).X, y+h/2)
		y += h + mindGapY*3
	}
	e.commit()
}

// connect draws an arrow from parent to child, unless one already binds them.
// The caller holds a transaction.
func (e *Editor) connect(parent, child doc.ID) doc.ID {
	for _, c := range e.MindmapChildren(parent) {
		if c == child {
			return ""
		}
	}
	ps, cs := e.Doc.Shapes[parent], e.Doc.Shapes[child]
	if ps == nil || cs == nil {
		return ""
	}
	a := doc.NewShape(doc.NewID("shape"), doc.TypeArrow)
	e.applyNext(a)
	// A mindmap's branches read as plain curves, not arrows with heads.
	a.HeadStart, a.HeadEnd = "none", "none"
	a.Parent, a.Index = e.Page, e.topIndex(e.Page)
	a.X, a.Y = ps.X, ps.Y
	a.Start, a.End = geo.V(0, 0), geo.V(1, 0)
	e.put(a)
	e.bindTerminal(a.ID, "start", parent, e.shapeCenter(parent), false)
	e.bindTerminal(a.ID, "end", child, e.shapeCenter(child), false)
	return a.ID
}

// removeEdge takes away the branch from parent to child.
func (e *Editor) removeEdge(parent, child doc.ID) {
	e.indexBindings()
	for _, b := range e.bindingsTo(parent) {
		if b.Terminal != "start" {
			continue
		}
		eb := e.bindingOf(b.From, "end")
		if eb == nil || eb.To != child {
			continue
		}
		arrow := b.From
		e.removeShape(arrow)
		e.removeBinding(b.ID)
		e.removeBinding(eb.ID)
		return
	}
}

// edgeAmong reports whether any arrow binds two of the named shapes.
func (e *Editor) edgeAmong(ids []doc.ID) bool {
	for _, id := range ids {
		for _, c := range e.MindmapChildren(id) {
			if slices.Contains(ids, c) {
				return true
			}
		}
	}
	return false
}

// createMindNode makes a mindmap box at a page point. The caller holds a
// transaction. It is a rounded rectangle, as the reference mindmap's topics
// are.
func (e *Editor) createMindNode(at geo.Vec) doc.ID {
	s := doc.NewShape(doc.NewID("shape"), doc.TypeGeo)
	s.Geo = "round"
	e.applyNext(s)
	// A mindmap reads better with straight edges than the board's sketchy
	// default.
	s.Dash = "solid"
	s.W, s.H = mindNodeW, mindNodeH
	e.place(s, at)
	e.put(s)
	return s.ID
}

// shapeCenter returns the center of a shape in page space.
func (e *Editor) shapeCenter(id doc.ID) geo.Vec {
	s := e.Doc.Shapes[id]
	if s == nil {
		return geo.Vec{}
	}
	return e.PageMat(id).Apply(e.Geometry(s).Bounds.Center())
}

// nodeCenter returns a node's center in page space.
func (e *Editor) nodeCenter(id doc.ID) geo.Vec {
	return e.PageMat(id).Apply(e.nodeBounds(id).Center())
}

// moveNodeCenter puts a node's center at a page point, unrotated and on the
// page.
func (e *Editor) moveNodeCenter(id doc.ID, cx, cy float64) {
	b := e.nodeBounds(id)
	e.update(id, func(s *doc.Shape) {
		s.Parent, s.Rotation = e.Page, 0
		s.X, s.Y = cx-b.W/2, cy-b.H/2
	})
}

// treeHeight returns how tall a node's subtree is, laid out.
func (e *Editor) treeHeight(id doc.ID) float64 {
	heights := map[doc.ID]float64{}
	onPath := map[doc.ID]bool{}
	return e.measureTree(id, heights, onPath)
}

func (e *Editor) measureTree(id doc.ID, heights map[doc.ID]float64, onPath map[doc.ID]bool) float64 {
	if h, ok := heights[id]; ok {
		return h
	}
	h := e.nodeBounds(id).H
	if onPath[id] {
		return h
	}
	onPath[id] = true
	total := 0.0
	for i, k := range e.MindmapChildren(id) {
		if i > 0 {
			total += mindGapY
		}
		total += e.measureTree(k, heights, onPath)
	}
	delete(onPath, id)
	if total > h {
		h = total
	}
	heights[id] = h
	return h
}

// nodeBounds returns a node's local bounds, or the zero box.
func (e *Editor) nodeBounds(id doc.ID) geo.Box {
	s := e.Doc.Shapes[id]
	if s == nil {
		return geo.Box{}
	}
	return e.Geometry(s).Bounds
}

// layoutTree lays a tree out with its root's center where it is.
func (e *Editor) layoutTree(root doc.ID) {
	c := e.nodeCenter(root)
	e.layoutTreeAt(root, c.X, c.Y)
}

// layoutTreeAt lays a tree out with the root's center at (cx, cy): the title in
// the middle, its top-level branches split between the right and the left by
// how many leaves each side holds, each subtree stacked and centered on its
// parent. Branches take a colour of their own and the bound arrows follow the
// nodes. It draws from the reference mindmap's layout, in mygo-native-ui.
func (e *Editor) layoutTreeAt(root doc.ID, cx, cy float64) float64 {
	if e.Doc.Shapes[root] == nil {
		return 0
	}
	e.begin()
	heights := map[doc.ID]float64{}
	onPath := map[doc.ID]bool{}
	measure := func(id doc.ID) float64 { return e.measureTree(id, heights, onPath) }

	// leaves counts a node's leaf ends, so the two sides can be balanced by
	// how much tree each carries.
	var leaves func(id doc.ID) int
	leaves = func(id doc.ID) int {
		kids := e.MindmapChildren(id)
		if len(kids) == 0 {
			return 1
		}
		n := 0
		for _, k := range kids {
			n += leaves(k)
		}
		return max(1, n)
	}

	// Split the root's branches between the right and the left, keeping their
	// order and giving the right as many leaves as the left, so the map is
	// centered on the title.
	type side struct {
		id  doc.ID
		col int
	}
	kids := e.MindmapChildren(root)
	total := 0
	for _, k := range kids {
		total += leaves(k)
	}
	var right, left []side
	acc := 0
	for i, k := range kids {
		w := leaves(k)
		if len(left) == 0 && (i == 0 || 2*acc+w <= total) {
			right = append(right, side{k, i})
			acc += w
		} else {
			left = append(left, side{k, i})
		}
	}

	placed := map[doc.ID]bool{}
	// place puts node id's near edge (the edge toward its parent) at near, its
	// center at y, and grows its subtree farther out, away from the title. dir
	// is +1 right, -1 left; col is the branch colour index; level is the depth
	// from the title.
	var place func(id doc.ID, near, y float64, dir, col, level int)
	place = func(id doc.ID, near, y float64, dir, col, level int) {
		if placed[id] {
			return
		}
		placed[id] = true
		b := e.nodeBounds(id)
		c := near + b.W/2
		next := near + b.W + mindGapX
		if dir < 0 {
			c = near - b.W/2
			next = near - b.W - mindGapX
		}
		e.moveNodeCenter(id, c, y)
		e.paintNode(id, col, level == 0)
		// The children stack and center on this node.
		ks := e.MindmapChildren(id)
		stack := 0.0
		for i, k := range ks {
			if i > 0 {
				stack += mindGapY
			}
			stack += measure(k)
		}
		top := y - stack/2
		for _, k := range ks {
			h := measure(k)
			place(k, next, top+h/2, dir, col, level+1)
			e.paintEdge(id, k, col, dir)
			top += h + mindGapY
		}
	}

	e.moveNodeCenter(root, cx, cy)
	e.paintNode(root, 0, true)
	rootB := e.nodeBounds(root)
	placeSide := func(s []side, near float64, dir int) {
		stack := 0.0
		for i, b := range s {
			if i > 0 {
				stack += mindGapY
			}
			stack += measure(b.id)
		}
		top := cy - stack/2
		for _, b := range s {
			h := measure(b.id)
			place(b.id, near, top+h/2, dir, b.col, 1)
			e.paintEdge(root, b.id, b.col, dir)
			top += h + mindGapY
		}
	}
	placeSide(right, cx+rootB.W/2+mindGapX, 1)
	placeSide(left, cx-rootB.W/2-mindGapX, -1)
	e.RefreshBoundaries()
	e.commit()
	return e.measureTree(root, heights, onPath)
}

// mindBranchColors orders the palette so that branches beside one another
// differ, standing in for the reference mindmap's golden-angle colours.
var mindBranchColors = []string{
	"blue", "orange", "violet", "green", "red",
	"light-blue", "yellow", "light-violet", "light-green", "light-red",
}

// mindBranchColor is branch k's palette colour. A step coprime with the
// palette length scatters neighbouring branches, as the golden angle does.
func mindBranchColor(k int) string {
	n := len(mindBranchColors)
	i := ((k % n) + n) % n
	return mindBranchColors[(i*3)%n]
}

// paintNode colours a mindmap node: the title and the text leaves take the
// plain ink, and a branch box the colour of its branch, as the reference
// mindmap keeps the colour in the branch's line.
func (e *Editor) paintNode(id doc.ID, col int, root bool) {
	s := e.Doc.Shapes[id]
	if s == nil {
		return
	}
	// A colour the user chose is left alone; only the plain ink is ours to
	// set, so a hand-picked colour sticks across layouts.
	if s.Color != "" && s.Color != "black" {
		return
	}
	color := "black"
	if !root && s.Type != doc.TypeText {
		color = mindBranchColor(col)
	}
	if s.Color != color {
		e.update(id, func(s *doc.Shape) { s.Color = color })
	}
}

// paintEdge colours the branch from parent to child, drops its arrowhead and
// curves it outward, so the map reads as one branch per colour fanning from
// its parent like the reference mindmap.
func (e *Editor) paintEdge(parent, child doc.ID, col, dir int) {
	a := e.edgeArrow(parent, child)
	if a == "" {
		return
	}
	color := mindBranchColor(col)
	// Bow the branch by how far the child sits, so a fan curves apart; the
	// generous sibling gap keeps those curves from crossing.
	pc, cc := e.nodeCenter(parent), e.nodeCenter(child)
	bend := (cc.Y - pc.Y) * 0.3
	bend = max(-48, min(48, bend))
	e.update(a, func(s *doc.Shape) {
		s.HeadStart, s.HeadEnd = "none", "none"
		s.Kind = "arc"
		s.Bend = bend
		// A colour the user chose is left alone.
		if s.Color == "" || s.Color == "black" {
			s.Color = color
		}
	})
	e.anchorEdge(a, parent, child, dir)
}

// anchorEdge binds a branch: it leaves the parent's near side at a point set
// by where the child sits, so a fan of branches does not bunch at one point,
// and lands on the child's near side.
func (e *Editor) anchorEdge(a, parent, child doc.ID, dir int) {
	sx, ex := 1.0, 0.0
	if dir < 0 {
		sx, ex = 0.0, 1.0
	}
	sy := 0.5
	if pb := e.PageBounds(parent); pb.H > 0 {
		sy = (e.nodeCenter(child).Y - pb.Y) / pb.H
		sy = max(0.12, min(0.88, sy))
	}
	e.bindAnchor(a, "start", parent, sx, sy)
	e.bindAnchor(a, "end", child, ex, 0.5)
}

// bindAnchor binds an arrow's end to a shape at a point of the shape's bounds,
// as fractions of its width and height.
func (e *Editor) bindAnchor(arrow doc.ID, term string, target doc.ID, ax, ay float64) {
	b := e.bindingOf(arrow, term)
	nb := &doc.Binding{ID: doc.NewID("binding"), Type: "arrow", From: arrow, To: target, Terminal: term, Anchor: geo.V(ax, ay), IsExact: true, IsPrecise: true}
	if b != nil {
		nb.ID, nb.Meta, nb.Raw = b.ID, b.Meta, b.Raw
	}
	e.putBinding(nb)
}

// mindLevel is how deep a node sits under its root, 0 for the root.
func (e *Editor) mindLevel(id doc.ID) int {
	level := 0
	seen := map[doc.ID]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		p := e.MindmapParent(id)
		if p == "" {
			break
		}
		level++
		id = p
	}
	return level
}

// createMindLeaf makes a node at a page point: a box for the title and its
// top-level branches, and plain text for anything deeper, as XMind draws its
// text leaves.
func (e *Editor) createMindLeaf(at geo.Vec, level int) doc.ID {
	if level <= 1 {
		return e.createMindNode(at)
	}
	s := doc.NewShape(doc.NewID("shape"), doc.TypeText)
	e.applyNext(s)
	s.Color = "black"
	// A leaf sits under its branch: smaller than the branch's own label.
	s.Size = "s"
	e.place(s, at)
	e.put(s)
	return s.ID
}

// edgeArrow returns the arrow binding parent to child, or "".
func (e *Editor) edgeArrow(parent, child doc.ID) doc.ID {
	e.indexBindings()
	for _, b := range e.bindingsTo(parent) {
		if b.Terminal != "start" {
			continue
		}
		eb := e.bindingOf(b.From, "end")
		if eb != nil && eb.To == child {
			return b.From
		}
	}
	return ""
}
