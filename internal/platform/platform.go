// Package platform isolates every place where Linux and macOS differ.
//
// The Go port is a single static binary for whichever GOOS it is built
// for, so there is nothing here about compiling for Apple Silicon: the
// toolchain handles that with GOARCH=arm64 and no cgo. What still
// differs is desktop integration.
//
// # What carried over from the Python version
//
//	Data directory     ~/Library/Application Support/Pomodoro on macOS,
//	                   $XDG_DATA_HOME/pomodoro on Linux.
//	Notifications      osascript on macOS, notify-send on Linux.
//	Shortcut labels    Cmd on macOS, Ctrl on Linux.
//	Hardware reporting Apple Silicon detection for diagnostics.
//
// # What did not
//
// Theme selection, font families, Tk version checking, HiDPI scaling
// and the macOS Cmd-Q handler were all Tkinter concerns. They have no
// equivalent here and were dropped rather than reimplemented as
// no-ops; whichever interface layer this port ends up with will have
// its own, different set of such concerns.
package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Compile-time platform identity. These are variables rather than
// constants so tests can exercise the other platform's branches, the
// same trick the Python suite used with IS_MAC and IS_LINUX.
var (
	GOOS   = runtime.GOOS
	GOARCH = runtime.GOARCH
)

// IsMac reports whether this build targets macOS.
func IsMac() bool { return GOOS == "darwin" }

// IsLinux reports whether this build targets Linux.
func IsLinux() bool { return GOOS == "linux" }

// IsAppleSilicon reports an arm64 macOS build.
//
// Unlike the Python version, which could only see the interpreter's
// architecture and so detected Rosetta by accident, a Go binary is
// compiled for one architecture. This is therefore a statement about
// the build, which is what matters for reporting.
func IsAppleSilicon() bool { return IsMac() && GOARCH == "arm64" }

// Summary is a one-line description of the environment, for diagnostics.
func Summary() string {
	name := GOOS
	switch GOOS {
	case "darwin":
		name = "macOS"
	case "linux":
		name = "Linux"
	}
	arch := GOARCH
	if IsAppleSilicon() {
		arch = "Apple Silicon"
	}
	return fmt.Sprintf("%s (%s), Go %s", name, arch, runtime.Version())
}

// userHomeDir is os.UserHomeDir, indirected so tests can supply a
// temporary home and can force the failure path.
var userHomeDir = os.UserHomeDir

// DataDir returns the per-user data directory for this platform,
// following each platform's own convention rather than dropping a
// dotfile in the home directory:
//
//	macOS  ~/Library/Application Support/Pomodoro
//	Linux  $XDG_DATA_HOME/pomodoro, or ~/.local/share/pomodoro
//
// The directory is created if it does not exist.
func DataDir() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("platform: locating home directory: %w", err)
	}

	var dir string
	if IsMac() {
		dir = filepath.Join(home, "Library", "Application Support", "Pomodoro")
	} else {
		// The XDG spec says the variable is honoured only when it holds
		// an absolute path.
		base := filepath.Join(home, ".local", "share")
		if xdg := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(xdg) {
			base = xdg
		}
		dir = filepath.Join(base, "pomodoro")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		// A read-only or sandboxed home must not prevent startup; fall
		// back to the home directory itself.
		return home, nil
	}
	return dir, nil
}

// HomeDir exposes the user's home directory, which the store needs in
// order to look for the legacy data file.
func HomeDir() (string, error) { return userHomeDir() }

// Modifier is the platform's primary shortcut key, for display.
func Modifier() string {
	if IsMac() {
		return "Cmd"
	}
	return "Ctrl"
}

// Accelerator renders a shortcut label, e.g. "Cmd+N" or "Ctrl+N".
func Accelerator(key string) string {
	return Modifier() + "+" + strings.ToUpper(key)
}

// notifyTimeout bounds how long a notification helper may run. The
// command is fire-and-forget, but an unbounded child process would
// outlive any reason to keep it.
const notifyTimeout = 10 * time.Second

// execCommand is exec.CommandContext, indirected for testing.
var execCommand = exec.CommandContext

// Notify shows a native desktop notification.
//
//	macOS  osascript -> Notification Center
//	Linux  notify-send -> whatever notification daemon is running
//
// It never blocks the caller and never returns an error for a missing
// notification daemon: that is a normal state on a minimal Linux
// install, and the interface layer will have sounded a bell and updated
// the window regardless. The bool reports whether a command was
// actually launched.
func Notify(title, message string) bool {
	var name string
	var args []string

	switch {
	case IsMac():
		// Escape double quotes so a task title cannot break out of the
		// AppleScript string literal.
		script := fmt.Sprintf(`display notification "%s" with title "%s"`,
			escapeAppleScript(message), escapeAppleScript(title))
		name, args = "osascript", []string{"-e", script}
	case IsLinux():
		name, args = "notify-send", []string{title, message}
	default:
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	cmd := execCommand(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		cancel()
		return false
	}
	// Reap the child in the background so it does not become a zombie,
	// and release the context when it finishes.
	go func() {
		defer cancel()
		_ = cmd.Wait()
	}()
	return true
}

// escapeAppleScript makes a string safe inside an AppleScript literal.
func escapeAppleScript(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
