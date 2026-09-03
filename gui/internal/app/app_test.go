package app

import (
	"strings"
	"testing"
	"time"

	"github.com/christopherwc/productivio/internal/core"
)

func TestEnvToday(t *testing.T) {
	want := core.NewDate(2026, time.September, 3)
	env := &Env{
		Now: func() time.Time { return time.Date(2026, time.September, 3, 14, 0, 0, 0, time.UTC) },
	}

	if got := env.Today(); got != want {
		t.Errorf("Today() = %v, want %v", got, want)
	}
}

func TestNewRealEnv(t *testing.T) {
	dir := t.TempDir()
	// Covers both platform.DataDir() branches deterministically,
	// regardless of which OS this test actually runs on.
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", dir)

	env, migrated, err := NewRealEnv()
	if err != nil {
		t.Fatalf("NewRealEnv() error = %v", err)
	}
	if migrated {
		t.Error("migrated = true in a fresh temp directory with no legacy file")
	}
	if env.Store == nil {
		t.Fatal("Store is nil")
	}
	if got := env.Store.Dir(); !strings.HasPrefix(got, dir) {
		t.Errorf("Store.Dir() = %q, want a path under %q", got, dir)
	}
	if env.Now == nil || env.Notify == nil || env.AfterFunc == nil {
		t.Error("NewRealEnv() left a nil hook")
	}
}
