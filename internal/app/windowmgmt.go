package app

import "github.com/egoist/mygo"

// tileSide names a half of the work area a window fills.
type tileSide int

const (
	tileLeft tileSide = iota
	tileRight
	tileTop
	tileBottom
)

// workArea is the room the display the window is on leaves for windows,
// without the menu bar and Dock.
func workArea(win *mygo.Window) mygo.Rectangle {
	if win == nil {
		return mygo.Rectangle{}
	}
	return mygo.Screen.DisplayMatching(win.Bounds()).WorkArea
}

// fillWindow fills the work area of the display the window is on.
func fillWindow(win *mygo.Window) {
	if rect := fillRect(workArea(win)); rect.Width > 0 && rect.Height > 0 {
		win.SetBounds(rect)
	}
}

// centerWindow keeps the window's size and puts it in the middle of the work
// area.
func centerWindow(win *mygo.Window) {
	if rect := centerRect(win.Bounds(), workArea(win)); rect.Width > 0 && rect.Height > 0 {
		win.SetBounds(rect)
	}
}

// tileWindow fills half the work area.
func tileWindow(win *mygo.Window, side tileSide) {
	if rect := tileRect(workArea(win), side); rect.Width > 0 && rect.Height > 0 {
		win.SetBounds(rect)
	}
}

// fillRect is the work area itself.
func fillRect(work mygo.Rectangle) mygo.Rectangle { return work }

// centerRect centers current in work, keeping its size where the work area has
// room for it.
func centerRect(current, work mygo.Rectangle) mygo.Rectangle {
	width := min(current.Width, work.Width)
	height := min(current.Height, work.Height)
	return mygo.Rectangle{
		X:      work.X + (work.Width-width)/2,
		Y:      work.Y + (work.Height-height)/2,
		Width:  width,
		Height: height,
	}
}

// tileRect is half the work area, split along the side's axis.
func tileRect(work mygo.Rectangle, side tileSide) mygo.Rectangle {
	halfW := work.Width / 2
	halfH := work.Height / 2
	switch side {
	case tileLeft:
		return mygo.Rectangle{X: work.X, Y: work.Y, Width: halfW, Height: work.Height}
	case tileRight:
		return mygo.Rectangle{X: work.X + halfW, Y: work.Y, Width: work.Width - halfW, Height: work.Height}
	case tileTop:
		return mygo.Rectangle{X: work.X, Y: work.Y, Width: work.Width, Height: halfH}
	default: // tileBottom
		return mygo.Rectangle{X: work.X, Y: work.Y + halfH, Width: work.Width, Height: work.Height - halfH}
	}
}
