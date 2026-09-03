// Command pomodoro is a focus timer with tasks, habits and project
// tracking.
//
// The Python original was a Tkinter desktop application. Go has no
// GUI toolkit in its standard library, so this port presents the same
// model through a command line instead. See the README for the
// reasoning and for what a graphical port would require.
//
// Usage:
//
//	pomodoro status                       today's summary
//	pomodoro start [flags]                run a work interval
//	pomodoro task list|add|done|rm        manage tasks
//	pomodoro project list|add|done        manage projects
//	pomodoro habit list|check             manage habits
//	pomodoro history [n]                  recent sessions
package main

import (
	"os"

	"github.com/your-username/pomodoro/internal/cli"
)

func main() {
	// All real work happens in the cli package so it can be tested
	// without spawning a process: main only wires up the real
	// environment and translates the result into an exit code.
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
