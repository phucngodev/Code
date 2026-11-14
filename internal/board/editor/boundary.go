package editor

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/phucngodev/code/internal/board/doc"
	"github.com/phucngodev/code/internal/board/geo"
)

// A boundary is XMind's: a bracket drawn beside a run of sibling subtopics,
// with a label on it, marking the group. It is ordinary shapes — a draw for the
// bracket and a box for the label — tied to the nodes they wrap by the
// bracket's Meta, so the layout can put them back as the tree moves.

const (
	boundaryKey  = "mindmapBoundary"
	boundaryPad  = 12.0 // room between the group and the bracket
	boundaryTick = 16.0 // how far the bracket's ends turn toward the group
)

// boundaryMeta is what a bracket's Meta holds.
type boundaryMeta struct {
	Nodes []doc.ID `json:"nodes"`
	Label doc.ID   `json:"label"`
}

// metaEnvelope wraps a boundary in the shape's Meta, which also holds what the
// file had.
type metaEnvelope struct {
	Boundary *boundaryMeta `json:"mindmapBoundary,omitempty"`
}

// AddBoundary draws a boundary beside the selected mindmap nodes. It needs at
// least two of them.
func (e *Editor) AddBoundary() doc.ID {
	var nodes []doc.ID
	for _, id := range e.Selection() {
		if s := e.Doc.Shapes[id]; s != nil && mindNode(s) {
			nodes = append(nodes, id)
		}
	}
	if len(nodes) < 2 {
		return ""
	}
	// Keep them in the order the tree lists, so the bracket hugs the group.
	sort.SliceStable(nodes, func(i, j int) bool {
		return e.Doc.Shapes[nodes[i]].Index < e.Doc.Shapes[nodes[j]].Index
	})
	color := e.branchColorOf(nodes[0])

	e.begin()
	brk := doc.NewShape(doc.NewID("shape"), doc.TypeLine)
	e.applyNext(brk)
	brk.Parent, brk.Index = e.Page, e.topIndex(e.Page)
	brk.Dash, brk.Color, brk.Spline = "solid", color, "line"
	brk.Points = make([]doc.LinePoint, 4)

	label := doc.NewShape(doc.NewID("shape"), doc.TypeGeo)
	e.applyNext(label)
	label.Parent, label.Index = e.Page, e.topIndex(e.Page)
	label.Geo, label.Dash, label.Fill, label.Color = "round", "solid", "fill", color
	label.LabelColor, label.Size = "white", "s"
	label.SetText("Summary")
	b := e.Measure("Summary", label.Font, labelSizeOf("s"), 0)
	label.W, label.H = max(80, b.W+40), max(30, b.H+20)

	raw, _ := json.Marshal(metaEnvelope{Boundary: &boundaryMeta{Nodes: nodes, Label: label.ID}})
	brk.Meta = raw
	e.put(brk)
	e.put(label)
	e.refreshBoundary(brk.ID)
	e.Select(brk.ID)
	e.commit()
	return brk.ID
}

// RemoveBoundary takes away a boundary and its label.
func (e *Editor) RemoveBoundary(id doc.ID) {
	m, ok := e.boundaryOf(id)
	if !ok {
		return
	}
	e.begin()
	if label := e.Doc.Shapes[m.Label]; label != nil {
		e.removeShape(m.Label)
	}
	e.removeShape(id)
	e.commit()
}

// RefreshBoundaries puts every boundary back beside the nodes it wraps. It
// runs after the layout and after a move, so a bracket follows its group.
func (e *Editor) RefreshBoundaries() {
	for _, id := range e.boundaryIDs() {
		e.refreshBoundary(id)
	}
}

// boundaryIDs lists the brackets by their Meta.
func (e *Editor) boundaryIDs() []doc.ID {
	var out []doc.ID
	for id, s := range e.Doc.Shapes {
		if s.Type != doc.TypeLine || len(s.Meta) == 0 {
			continue
		}
		if _, ok := decodeBoundary(s.Meta); ok {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// boundaryOf returns a bracket's meta.
func (e *Editor) boundaryOf(id doc.ID) (boundaryMeta, bool) {
	s := e.Doc.Shapes[id]
	if s == nil {
		return boundaryMeta{}, false
	}
	m, ok := decodeBoundary(s.Meta)
	if !ok {
		return boundaryMeta{}, false
	}
	return m, true
}

func decodeBoundary(raw json.RawMessage) (boundaryMeta, bool) {
	if len(raw) == 0 {
		return boundaryMeta{}, false
	}
	var env metaEnvelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Boundary == nil {
		return boundaryMeta{}, false
	}
	return *env.Boundary, true
}

// refreshBoundary recomputes one bracket and its label from the nodes it
// wraps: a bracket beside them on the side toward their parent, with the label
// in the gap between the group and the bracket, as XMind draws it.
func (e *Editor) refreshBoundary(id doc.ID) {
	m, ok := e.boundaryOf(id)
	if !ok || e.Doc.Shapes[id] == nil {
		return
	}
	nodes := make([]doc.ID, 0, len(m.Nodes))
	var box geo.Box
	for _, n := range m.Nodes {
		if s := e.Doc.Shapes[n]; s != nil && mindNode(s) {
			nodes = append(nodes, n)
			box = union(box, e.PageBounds(n))
		}
	}
	if len(nodes) < 2 {
		if label := e.Doc.Shapes[m.Label]; label != nil {
			e.removeShape(m.Label)
		}
		e.removeShape(id)
		return
	}
	m.Nodes = nodes

	// The group's side of its parent sets which edge the bracket takes.
	groupRight := true
	if p := e.MindmapParent(nodes[0]); p != "" {
		groupRight = e.nodeCenter(nodes[0]).X >= e.nodeCenter(p).X
	}

	lw := 96.0
	if label := e.Doc.Shapes[m.Label]; label != nil {
		lw = label.W
	}
	// The order is group, then the bracket, then the label: the bracket cups
	// the group from the side away from its parent, and the label follows it.
	var x, turn, labelX float64
	if groupRight {
		x, turn = box.X+box.W+boundaryPad, -boundaryTick
		labelX = x + lw/2 + boundaryPad
	} else {
		x, turn = box.X-boundaryPad, boundaryTick
		labelX = x - lw/2 - boundaryPad
	}
	top, bottom := box.Y, box.Y+box.H
	e.setBracket(e.Doc.Shapes[id], []geo.Vec{
		{X: x + turn, Y: top},
		{X: x, Y: top},
		{X: x, Y: bottom},
		{X: x + turn, Y: bottom},
	})

	if label := e.Doc.Shapes[m.Label]; label != nil {
		mid := (top + bottom) / 2
		e.update(label.ID, func(s *doc.Shape) {
			s.Parent = e.Page
			s.X, s.Y = labelX-label.W/2, mid-label.H/2
		})
	}
	raw, _ := json.Marshal(metaEnvelope{Boundary: &m})
	e.update(id, func(s *doc.Shape) { s.Meta = raw })
}

// setBracket writes a bracket's polyline, keeping the shape's position at the
// polyline's top-left. It is a line, so its corners stay sharp as a [ or ].
func (e *Editor) setBracket(s *doc.Shape, pts []geo.Vec) {
	minX, minY := math.Inf(1), math.Inf(1)
	for _, p := range pts {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
	}
	local := make([]doc.LinePoint, len(pts))
	for i, p := range pts {
		local[i] = doc.LinePoint{X: p.X - minX, Y: p.Y - minY}
	}
	e.update(s.ID, func(c *doc.Shape) {
		c.Parent = e.Page
		c.X, c.Y, c.Spline = minX, minY, "line"
		c.Points = local
	})
}

// branchColorOf is the colour of the top-level branch a node belongs to.
func (e *Editor) branchColorOf(id doc.ID) string {
	cur := id
	for {
		p := e.MindmapParent(cur)
		if p == "" {
			break
		}
		if e.MindmapParent(p) == "" {
			// p is the root, so cur is a top-level branch.
			break
		}
		cur = p
	}
	if s := e.Doc.Shapes[cur]; s != nil && s.Color != "" {
		return s.Color
	}
	return "red"
}

func union(a, b geo.Box) geo.Box {
	if a.W == 0 && a.H == 0 {
		return b
	}
	x := math.Min(a.X, b.X)
	y := math.Min(a.Y, b.Y)
	r := math.Max(a.X+a.W, b.X+b.W)
	bt := math.Max(a.Y+a.H, b.Y+b.H)
	return geo.Box{X: x, Y: y, W: r - x, H: bt - y}
}
