// Command cde reports what a coding agent is doing to a running Code, over its
// agent socket. Code sets CODE_SOCKET in the shells it starts; run outside Code
// it finds the newest running one.
//
//	cde event needsInput --agent claude --detail "Approve this command?"
//	cde hook codex < hook.json
//	cde launch claude --resume
//	cde ping
package main

import (
	"os"

	"github.com/phucngodev/code/internal/agent"
)

func main() {
	os.Exit(agent.RunCde(os.Args[1:], os.Environ(), os.Stdin, os.Stdout, os.Stderr))
}
