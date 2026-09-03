package platform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// asPlatform runs fn with GOOS and GOARCH forced to the given values,
// restoring them afterwards.
//
// This is what lets a Linux CI machine exercise the macOS branches and
// vice versa. Without it, half of this package would be a permanent
// dead spot in the coverage report on either platform.
func asPlatform(t *testing.T, goos, goarch string, fn func()) {
	t.Helper()
	origOS, origArch := GOOS, GOARCH
	GOOS, GOARCH = goos, goarch
	defer func() { GOOS, GOARCH = origOS, origArch }()
	fn()
}

func TestPlatformIdentity(t *testing.T) {
	t.Run("flags are mutually exclusive", func(t *testing.T) {
		if IsMac() && IsLinux() {
			t.Error("cannot be both macOS and Linux")
		}
	})

	t.Run("Apple Silicon detection", func(t *testing.T) {
		tests := []struct {
			goos, goarch string
			want         bool
			name         string
		}{
			{"darwin", "arm64", true, "M-series Mac"},
			{"darwin", "amd64", false, "Intel Mac"},
			{"linux", "arm64", false, "arm64 Linux is not Apple Silicon"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				asPlatform(t, tc.goos, tc.goarch, func() {
					if got := IsAppleSilicon(); got != tc.want {
						t.Errorf("IsAppleSilicon() = %v, want %v", got, tc.want)
					}
				})
			})
		}
	})

	t.Run("Summary names each platform", func(t *testing.T) {
		tests := []struct {
			goos, goarch, want string
		}{
			{"darwin", "arm64", "macOS (Apple Silicon)"},
			{"darwin", "amd64", "macOS (amd64)"},
			{"linux", "amd64", "Linux (amd64)"},
			{"plan9", "386", "plan9 (386)"},
		}
		for _, tc := range tests {
			asPlatform(t, tc.goos, tc.goarch, func() {
				got := Summary()
				if !strings.HasPrefix(got, tc.want) {
					t.Errorf("Summary() = %q, want it to start with %q", got, tc.want)
				}
				if !strings.Contains(got, runtime.Version()) {
					t.Errorf("Summary() = %q, want the Go version in it", got)
				}
			})
		}
	})
}

func TestDataDir(t *testing.T) {
	// Point the home directory at a temporary one so no test can touch
	// the real user's data.
	withHome := func(t *testing.T, home string, fn func()) {
		t.Helper()
		orig := userHomeDir
		userHomeDir = func() (string, error) { return home, nil }
		defer func() { userHomeDir = orig }()
		fn()
	}

	t.Run("macOS uses Application Support", func(t *testing.T) {
		home := t.TempDir()
		withHome(t, home, func() {
			asPlatform(t, "darwin", "arm64", func() {
				got, err := DataDir()
				if err != nil {
					t.Fatal(err)
				}
				want := filepath.Join(home, "Library", "Application Support", "Pomodoro")
				if got != want {
					t.Errorf("DataDir() = %q, want %q", got, want)
				}
				if info, err := os.Stat(got); err != nil || !info.IsDir() {
					t.Errorf("directory was not created: %v", err)
				}
			})
		})
	})

	t.Run("Linux defaults to .local/share", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("XDG_DATA_HOME", "")
		withHome(t, home, func() {
			asPlatform(t, "linux", "amd64", func() {
				got, _ := DataDir()
				want := filepath.Join(home, ".local", "share", "pomodoro")
				if got != want {
					t.Errorf("DataDir() = %q, want %q", got, want)
				}
			})
		})
	})

	t.Run("Linux honours XDG_DATA_HOME", func(t *testing.T) {
		home := t.TempDir()
		xdg := t.TempDir()
		t.Setenv("XDG_DATA_HOME", xdg)
		withHome(t, home, func() {
			asPlatform(t, "linux", "amd64", func() {
				got, _ := DataDir()
				if want := filepath.Join(xdg, "pomodoro"); got != want {
					t.Errorf("DataDir() = %q, want %q", got, want)
				}
			})
		})
	})

	t.Run("a relative XDG path is ignored", func(t *testing.T) {
		// The XDG spec says the variable counts only when absolute.
		home := t.TempDir()
		t.Setenv("XDG_DATA_HOME", "not/absolute")
		withHome(t, home, func() {
			asPlatform(t, "linux", "amd64", func() {
				got, _ := DataDir()
				want := filepath.Join(home, ".local", "share", "pomodoro")
				if got != want {
					t.Errorf("DataDir() = %q, want %q", got, want)
				}
			})
		})
	})

	t.Run("falls back to home when the directory cannot be created", func(t *testing.T) {
		// A read-only or sandboxed home must not prevent startup.
		blocked := filepath.Join(t.TempDir(), "blocker")
		if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		withHome(t, blocked, func() {
			asPlatform(t, "linux", "amd64", func() {
				got, err := DataDir()
				if err != nil {
					t.Fatalf("should degrade, not fail: %v", err)
				}
				if got != blocked {
					t.Errorf("DataDir() = %q, want the home fallback %q", got, blocked)
				}
			})
		})
	})

	t.Run("an unknown home directory is an error", func(t *testing.T) {
		orig := userHomeDir
		userHomeDir = func() (string, error) { return "", errors.New("no home") }
		defer func() { userHomeDir = orig }()

		if _, err := DataDir(); err == nil {
			t.Error("expected an error when the home directory is unknown")
		}
		if _, err := HomeDir(); err == nil {
			t.Error("HomeDir should propagate the error")
		}
	})

	t.Run("HomeDir reports the real home", func(t *testing.T) {
		home := t.TempDir()
		withHome(t, home, func() {
			if got, err := HomeDir(); err != nil || got != home {
				t.Errorf("HomeDir() = %q, %v", got, err)
			}
		})
	})
}

func TestShortcutLabels(t *testing.T) {
	tests := []struct {
		goos, modifier, accelerator string
	}{
		{"darwin", "Cmd", "Cmd+N"},
		{"linux", "Ctrl", "Ctrl+N"},
	}
	for _, tc := range tests {
		t.Run(tc.goos, func(t *testing.T) {
			asPlatform(t, tc.goos, "amd64", func() {
				if got := Modifier(); got != tc.modifier {
					t.Errorf("Modifier() = %q, want %q", got, tc.modifier)
				}
				if got := Accelerator("n"); got != tc.accelerator {
					t.Errorf("Accelerator(\"n\") = %q, want %q", got, tc.accelerator)
				}
			})
		})
	}
}

func TestNotify(t *testing.T) {
	// Capture the command instead of running it, so tests never spawn
	// a real osascript or notify-send.
	type captured struct {
		name string
		args []string
	}
	withFakeExec := func(t *testing.T, fail bool, fn func(*captured)) {
		t.Helper()
		var got captured
		orig := execCommand
		execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			got = captured{name, args}
			if fail {
				return exec.CommandContext(ctx, "/nonexistent/binary")
			}
			// "true" exists on both Linux and macOS and exits at once.
			return exec.CommandContext(ctx, "true")
		}
		defer func() { execCommand = orig }()
		fn(&got)
	}

	t.Run("macOS uses osascript", func(t *testing.T) {
		withFakeExec(t, false, func(got *captured) {
			asPlatform(t, "darwin", "arm64", func() {
				if !Notify("Title", "Body") {
					t.Error("Notify should report success")
				}
			})
			if got.name != "osascript" {
				t.Errorf("command = %q, want osascript", got.name)
			}
			if len(got.args) != 2 || !strings.Contains(got.args[1], "display notification") {
				t.Errorf("args = %v", got.args)
			}
		})
	})

	t.Run("macOS escapes quotes in the AppleScript", func(t *testing.T) {
		// An unescaped quote in a task title would break out of the
		// script's string literal.
		withFakeExec(t, false, func(got *captured) {
			asPlatform(t, "darwin", "arm64", func() {
				Notify(`He said "hi"`, `Task "urgent" done`)
			})
			script := got.args[1]
			if !strings.Contains(script, `\"hi\"`) || !strings.Contains(script, `\"urgent\"`) {
				t.Errorf("quotes were not escaped: %s", script)
			}
		})
	})

	t.Run("Linux uses notify-send", func(t *testing.T) {
		withFakeExec(t, false, func(got *captured) {
			asPlatform(t, "linux", "amd64", func() {
				if !Notify("Title", "Body") {
					t.Error("Notify should report success")
				}
			})
			// Give the reaper goroutine a moment to collect the child.
			// The stub command exits immediately; without this the test
			// can finish first and leave the reap path unexercised.
			time.Sleep(200 * time.Millisecond)
			if got.name != "notify-send" {
				t.Errorf("command = %q, want notify-send", got.name)
			}
			if len(got.args) != 2 || got.args[0] != "Title" || got.args[1] != "Body" {
				t.Errorf("args = %v", got.args)
			}
		})
	})

	t.Run("a missing notification daemon degrades quietly", func(t *testing.T) {
		// No daemon is a normal state on a minimal Linux install; the
		// caller will have updated the window regardless.
		withFakeExec(t, true, func(*captured) {
			asPlatform(t, "linux", "amd64", func() {
				if Notify("Title", "Body") {
					t.Error("Notify should report failure when the command cannot start")
				}
			})
		})
	})

	t.Run("an unsupported platform reports false", func(t *testing.T) {
		asPlatform(t, "windows", "amd64", func() {
			if Notify("Title", "Body") {
				t.Error("Notify should report false on an unsupported platform")
			}
		})
	})
}
