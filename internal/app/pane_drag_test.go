package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSuperDragMovesPane checks that holding Super and dragging a pane moves
// it onto another pane, with no grip to press. The tester's pointer events do
// not carry modifiers, so Super is re-asserted before each one, as the frame
// the drag reads its source state is built while it is held.
func TestSuperDragMovesPane(t *testing.T) {
	w := liveWindow(t, nil)
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()

	before := w.store.Panes()
	if len(before) != 2 {
		t.Fatalf("panes = %d, want 2", len(before))
	}
	hold := func() { tester.HoldModifiers(ui.Super); tester.Frame() }
	hold()
	tester.Press(200, 300)
	hold()
	tester.Move(360, 300)
	hold()
	tester.Move(700, 300)
	hold()
	tester.Release(700, 300)
	tester.Frame()
	tester.Frame()

	after := w.store.Panes()
	if len(after) != 2 {
		t.Fatalf("panes after drag = %d, want 2", len(after))
	}
	if after[0] == before[0] && after[1] == before[1] {
		t.Fatal("a Super+drag should move the pane, but the order is unchanged")
	}
}

// TestPlainDragDoesNotMovePane checks that a drag without Super selects in the
// terminal instead of moving the pane.
func TestPlainDragDoesNotMovePane(t *testing.T) {
	w := liveWindow(t, nil)
	tester := ui.NewTester(w.view, 900, 600)
	tester.Frame()

	before := w.store.Panes()
	tester.Press(200, 300)
	tester.Move(360, 300)
	tester.Move(700, 300)
	tester.Release(700, 300)
	tester.Frame()
	tester.Frame()

	after := w.store.Panes()
	if after[0] != before[0] || after[1] != before[1] {
		t.Fatal("a drag without Super should not move the pane")
	}
}
