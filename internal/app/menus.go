package app

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"

	"github.com/phucngodev/code/internal/model"
)

// appKey names a menu shortcut that must not shadow a Control key a program
// uses, as vim and a shell use Control+F, Control+P and Control+C on Linux:
// Command on macOS, Control+Shift elsewhere.
func appKey(key string) string {
	if runtime.GOOS == "darwin" {
		return "Cmd+" + key
	}
	return "Shift+CmdOrCtrl+" + key
}

// buildMenu builds the application menu. Menu commands act on the window the
// event came from, falling back to the focused one.
func (a *App) buildMenu() *mygo.Menu {
	item := func(label, accelerator string, fn func(w *WindowState)) *mygo.MenuItem {
		return &mygo.MenuItem{
			Label:       label,
			Accelerator: accelerator,
			Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				if w := a.windowFor(win); w != nil {
					fn(w)
				}
			},
		}
	}
	windowItem := func(label, accelerator string, fn func(*mygo.Window)) *mygo.MenuItem {
		return &mygo.MenuItem{
			Label:       label,
			Accelerator: accelerator,
			Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				if win == nil {
					win = mygo.FocusedWindow()
				}
				if win != nil {
					fn(win)
				}
			},
		}
	}

	appMenu := []*mygo.MenuItem{
		{Role: mygo.RoleAbout},
		mygo.Separator(),
		{Label: "Settings…", Accelerator: acc.settings, Click: func(*mygo.MenuItem, *mygo.Window) { a.ShowSettings() }},
		{Label: "Reload Configuration", Accelerator: acc.reloadConfig, Click: func(*mygo.MenuItem, *mygo.Window) { a.ReloadConfig() }},
		mygo.Separator(),
		{Role: mygo.RoleHide},
		{Role: mygo.RoleHideOthers},
		{Role: mygo.RoleUnhide},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}

	fileMenu := []*mygo.MenuItem{
		{Label: "New Window", Accelerator: acc.newWindow, Click: func(*mygo.MenuItem, *mygo.Window) { a.newWindow(nil) }},
		item("New Workspace", "", func(w *WindowState) { w.newWorkspace("") }),
		item("New Tab", acc.newTab, func(w *WindowState) { w.newTab("") }),
		mygo.Separator(),
		item("Close Pane", acc.closePane, func(w *WindowState) {
			if s := w.focusedSurface(); s != nil {
				w.closeSurface(s)
			}
		}),
		item("Close Tab", acc.closeTab, func(w *WindowState) {
			if ws := w.store.Selected(); ws != nil {
				if tab := ws.SelectedTab(); tab != nil {
					w.closeTab(tab)
				}
			}
		}),
		{Label: "Close Window", Accelerator: acc.closeWindow, Click: func(_ *mygo.MenuItem, win *mygo.Window) {
			if win != nil {
				win.Close()
			}
		}},
		mygo.Separator(),
		item("Split Right", acc.splitRight, func(w *WindowState) { w.splitFocused(model.DirRight) }),
		item("Split Down", acc.splitDown, func(w *WindowState) { w.splitFocused(model.DirDown) }),
	}

	editMenu := []*mygo.MenuItem{
		{Role: mygo.RoleCopy, Accelerator: acc.copy},
		{Role: mygo.RolePaste, Accelerator: acc.paste},
		{Role: mygo.RoleSelectAll, Accelerator: acc.selectAll},
		mygo.Separator(),
		item("Find…", acc.find, func(w *WindowState) { w.openFind() }),
		item("Find Next", acc.findNext, func(w *WindowState) { w.searchNext() }),
		item("Find Previous", acc.findPrevious, func(w *WindowState) { w.searchPrevious() }),
	}

	viewMenu := []*mygo.MenuItem{
		item("Toggle Sidebar", acc.toggleSidebar, func(w *WindowState) { w.toggleSidebar() }),
		item("Go to…", acc.goTo, func(w *WindowState) { w.openPalette() }),
		item("Zoom Pane", acc.zoomPane, func(w *WindowState) { w.toggleZoom() }),
		mygo.Separator(),
		windowItem("Enter Full Screen", "", func(win *mygo.Window) { win.SetFullScreen(!win.IsFullScreen()) }),
	}

	workspaceMenu := []*mygo.MenuItem{
		item("Previous Workspace", acc.prevWorkspace, func(w *WindowState) { w.cycleWorkspace(-1) }),
		item("Next Workspace", acc.nextWorkspace, func(w *WindowState) { w.cycleWorkspace(1) }),
		item("Rename Workspace", acc.renameWorkspace, func(w *WindowState) {
			if ws := w.store.Selected(); ws != nil {
				w.beginRename(ws)
			}
		}),
		mygo.Separator(),
	}
	for i := 1; i <= 9; i++ {
		i := i
		accelerator := workspaceAccelerator(i)
		label := fmt.Sprintf("Switch to Workspace %d", i)
		if i == 9 {
			label = "Last Workspace"
		}
		workspaceMenu = append(workspaceMenu, item(label, accelerator, func(w *WindowState) {
			if i == 9 {
				w.selectWorkspaceIndex(len(w.store.Workspaces) - 1)
			} else {
				w.selectWorkspaceIndex(i - 1)
			}
		}))
	}
	workspaceMenu = append(workspaceMenu, mygo.Separator())
	for i := 1; i <= 9; i++ {
		i := i
		label := fmt.Sprintf("Switch to Tab %d", i)
		if i == 9 {
			label = "Last Tab"
		}
		workspaceMenu = append(workspaceMenu, item(label, tabAccelerator(i), func(w *WindowState) {
			if i == 9 {
				if ws := w.store.Selected(); ws != nil {
					w.selectTabIndex(len(ws.Tabs) - 1)
				}
			} else {
				w.selectTabIndex(i - 1)
			}
		}))
	}

	windowMenu := a.windowMenuItems()

	helpMenu := []*mygo.MenuItem{
		{Label: "Keyboard Shortcuts", Click: func(*mygo.MenuItem, *mygo.Window) { a.ShowShortcuts() }},
	}

	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Code", Submenu: appMenu},
		{Label: "File", Submenu: fileMenu},
		{Label: "Edit", Submenu: editMenu},
		{Label: "View", Submenu: viewMenu},
		{Label: "Workspace", Submenu: workspaceMenu},
		{Label: "Window", Submenu: windowMenu},
		{Label: "Help", Submenu: helpMenu},
	})
}

// windowMenuItems is the Window menu. Its window management commands and
// shortcuts follow macOS's own (System Settings > Keyboard > Keyboard
// Shortcuts > Windows), so pressing them manages the window here.
func (a *App) windowMenuItems() []*mygo.MenuItem {
	windowItem := func(label, accelerator string, fn func(*mygo.Window)) *mygo.MenuItem {
		return &mygo.MenuItem{
			Label:       label,
			Accelerator: accelerator,
			Click: func(_ *mygo.MenuItem, win *mygo.Window) {
				if win == nil {
					win = mygo.FocusedWindow()
				}
				if win != nil {
					fn(win)
				}
			},
		}
	}
	return []*mygo.MenuItem{
		{Role: mygo.RoleMinimize, Accelerator: acc.minimize},
		{Role: mygo.RoleZoom},
		mygo.Separator(),
		windowItem("Fill", acc.fill, func(win *mygo.Window) { a.manageWindow(win, func() { fillWindow(win) }) }),
		windowItem("Center", acc.center, func(win *mygo.Window) { a.manageWindow(win, func() { centerWindow(win) }) }),
		windowItem("Return to Previous Size", acc.previousSize, func(win *mygo.Window) { a.restorePreviousSize(win) }),
		mygo.Separator(),
		windowItem("Tile Left Half", acc.tileLeft, func(win *mygo.Window) { a.manageWindow(win, func() { tileWindow(win, tileLeft) }) }),
		windowItem("Tile Right Half", acc.tileRight, func(win *mygo.Window) { a.manageWindow(win, func() { tileWindow(win, tileRight) }) }),
		windowItem("Tile Top Half", acc.tileTop, func(win *mygo.Window) { a.manageWindow(win, func() { tileWindow(win, tileTop) }) }),
		windowItem("Tile Bottom Half", acc.tileBottom, func(win *mygo.Window) { a.manageWindow(win, func() { tileWindow(win, tileBottom) }) }),
		mygo.Separator(),
		{Role: mygo.RoleFront},
	}
}
