//go:build unix

package agent

import "syscall"

// Exec replaces the process with the agent's program.
func (l Launcher) Exec() error {
	env := make([]string, 0, len(l.Environment))
	for key, value := range l.Environment {
		env = append(env, key+"="+value)
	}
	return syscall.Exec(l.Executable, l.Arguments, env)
}
