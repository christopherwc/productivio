package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

func TestHistoryRowText(t *testing.T) {
	start := time.Date(2026, time.September, 2, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		session *core.Session
		want    string
	}{
		{
			"a session credited to a task and project",
			core.NewSession(start, start.Add(25*time.Minute), 25,
				&core.Task{ID: "t1", Title: "Design"}, &core.Project{ID: "p1", Name: "Website"}),
			"2026-09-02  09:00  ·  25m  ·  Design · Website",
		},
		{
			"an unattributed session shows dashes",
			core.NewSession(start, start.Add(5*time.Minute), 5, nil, nil),
			"2026-09-02  09:00  ·  5m  ·  - · -",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := historyRowText(tt.session); got != tt.want {
				t.Errorf("historyRowText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewHistoryTab(t *testing.T) {
	// A regression guard for the tab-building code path with an actual
	// session logged. (The per-session rows live inside a widget.List,
	// whose items are not instantiated without a real render — see
	// historyRowText above for the content logic those rows use,
	// tested directly.)
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	start := time.Now()
	store.sessions = core.Sessions{core.NewSession(start, start.Add(25*time.Minute), 25, nil, nil)}

	env := &app.Env{Store: store, Now: func() time.Time { return time.Now() }}
	if content := NewHistoryTab(env); content == nil {
		t.Error("NewHistoryTab() = nil")
	}
}
