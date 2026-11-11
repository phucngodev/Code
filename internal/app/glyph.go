package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/phucngodev/code/internal/agent"
)

// agentGlyph draws an agent's state mark: a turning arc while it works, a dot
// when it waits, a check when a turn finished, and nothing when idle.
func agentGlyph(c *ui.Context, indicator agent.Indicator) {
	switch indicator {
	case agent.IndicatorWorking:
		ui.Spinner(c).Size(12, 12).TextColor(ui.RGB(0x0A, 0x84, 0xFF))
	case agent.IndicatorWaiting:
		ui.Box(c).Size(7, 7).Radius(4).Background(ui.RGB(0xFF, 0xCC, 0x00))
	case agent.IndicatorDone:
		ui.Icon(c, checkIcon).FontSize(12).TextColor(ui.RGB(0x28, 0xCD, 0x41))
	}
}
