package app

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/terminal.svg
var terminalIconSVG []byte

//go:embed icons/sparkle.svg
var sparkleIconSVG []byte

//go:embed icons/gear.svg
var gearIconSVG []byte

//go:embed icons/palette.svg
var paletteIconSVG []byte

//go:embed icons/bell.svg
var bellIconSVG []byte

//go:embed icons/gears.svg
var gearsIconSVG []byte

//go:embed icons/search.svg
var searchIconSVG []byte

//go:embed icons/grid.svg
var gridIconSVG []byte

//go:embed icons/split-right.svg
var splitRightIconSVG []byte

//go:embed icons/split-down.svg
var splitDownIconSVG []byte

//go:embed icons/check.svg
var checkIconSVG []byte

//go:embed icons/chevron.svg
var chevronIconSVG []byte

//go:embed icons/sidebar.svg
var sidebarIconSVG []byte

//go:embed icons/plus.svg
var plusIconSVG []byte

//go:embed icons/folder-plus.svg
var folderPlusIconSVG []byte

//go:embed icons/pencil.svg
var pencilIconSVG []byte

//go:embed icons/notebook.svg
var notebookIconSVG []byte

var (
	terminalIcon   = ui.MustParseSVG(terminalIconSVG)
	sparkleIcon    = ui.MustParseSVG(sparkleIconSVG)
	gearIcon       = ui.MustParseSVG(gearIconSVG)
	paletteIcon    = ui.MustParseSVG(paletteIconSVG)
	bellIcon       = ui.MustParseSVG(bellIconSVG)
	gearsIcon      = ui.MustParseSVG(gearsIconSVG)
	searchIcon     = ui.MustParseSVG(searchIconSVG)
	gridIcon       = ui.MustParseSVG(gridIconSVG)
	splitRightIcon = ui.MustParseSVG(splitRightIconSVG)
	splitDownIcon  = ui.MustParseSVG(splitDownIconSVG)
	checkIcon      = ui.MustParseSVG(checkIconSVG)
	chevronIcon    = ui.MustParseSVG(chevronIconSVG)
	sidebarIcon    = ui.MustParseSVG(sidebarIconSVG)
	plusIcon       = ui.MustParseSVG(plusIconSVG)
	folderPlusIcon = ui.MustParseSVG(folderPlusIconSVG)
	pencilIcon     = ui.MustParseSVG(pencilIconSVG)
	notebookIcon   = ui.MustParseSVG(notebookIconSVG)
)
