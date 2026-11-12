package app

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestTileRect(t *testing.T) {
	work := mygo.Rectangle{X: 10, Y: 25, Width: 1000, Height: 800}
	cases := []struct {
		side tileSide
		want mygo.Rectangle
	}{
		{tileLeft, mygo.Rectangle{X: 10, Y: 25, Width: 500, Height: 800}},
		{tileRight, mygo.Rectangle{X: 510, Y: 25, Width: 500, Height: 800}},
		{tileTop, mygo.Rectangle{X: 10, Y: 25, Width: 1000, Height: 400}},
		{tileBottom, mygo.Rectangle{X: 10, Y: 425, Width: 1000, Height: 400}},
	}
	for _, tc := range cases {
		if got := tileRect(work, tc.side); got != tc.want {
			t.Fatalf("tileRect(%v) = %+v, want %+v", tc.side, got, tc.want)
		}
	}
}

func TestTileRectOddSizeCoversWhole(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 0, Width: 1001, Height: 801}
	left := tileRect(work, tileLeft)
	right := tileRect(work, tileRight)
	if left.X+left.Width != right.X || right.X+right.Width != work.Width {
		t.Fatalf("halves do not tile: %+v %+v", left, right)
	}
	top := tileRect(work, tileTop)
	bottom := tileRect(work, tileBottom)
	if top.Y+top.Height != bottom.Y || bottom.Y+bottom.Height != work.Height {
		t.Fatalf("halves do not tile: %+v %+v", top, bottom)
	}
}

func TestCenterRectKeepsSize(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 0, Width: 1000, Height: 800}
	got := centerRect(mygo.Rectangle{X: 0, Y: 0, Width: 400, Height: 300}, work)
	if got != (mygo.Rectangle{X: 300, Y: 250, Width: 400, Height: 300}) {
		t.Fatalf("centerRect = %+v", got)
	}
}

func TestCenterRectClampsToWorkArea(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 0, Width: 500, Height: 400}
	got := centerRect(mygo.Rectangle{X: 0, Y: 0, Width: 1000, Height: 300}, work)
	if got.Width != 500 || got.Height != 300 || got.X != 0 || got.Y != 50 {
		t.Fatalf("centerRect = %+v", got)
	}
}

func TestWindowMenuFollowsMacOSShortcuts(t *testing.T) {
	want := map[string]string{
		"Fill":                    "Ctrl+Shift+F",
		"Center":                  "Ctrl+Shift+X",
		"Return to Previous Size": "Ctrl+Shift+R",
		"Tile Left Half":          "Ctrl+Shift+Left",
		"Tile Right Half":         "Ctrl+Shift+Right",
		"Tile Top Half":           "Ctrl+Shift+Up",
		"Tile Bottom Half":        "Ctrl+Shift+Down",
	}
	got := map[string]string{}
	for _, item := range (&App{}).windowMenuItems() {
		if item.Accelerator != "" {
			got[item.Label] = item.Accelerator
		}
	}
	for label, accelerator := range want {
		if got[label] != accelerator {
			t.Errorf("%s accelerator = %q, want %q", label, got[label], accelerator)
		}
	}
}
