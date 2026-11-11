//go:build !unix

package agent

import "errors"

// Exec is not supported on this platform; the shim and launch path are for
// Unix-like systems.
func (l Launcher) Exec() error {
	return errors.New("cde launch is not supported on this platform")
}
