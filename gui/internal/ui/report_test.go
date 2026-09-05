package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

func TestReportRowText(t *testing.T) {
	tests := []struct {
		name  string
		total core.ProjectTotal
		want  string
	}{
		{
			"a named project with several sessions",
			core.ProjectTotal{ProjectName: "Website", Sessions: 3, Minutes: 75},
			"Website  ·  3 sessions  ·  1h 15m",
		},
		{
			"singular session",
			core.ProjectTotal{ProjectName: "Website", Sessions: 1, Minutes: 25},
			"Website  ·  1 session  ·  25m",
		},
		{
			"unattributed work has no project name",
			core.ProjectTotal{Sessions: 1, Minutes: 25},
			"-  ·  1 session  ·  25m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reportRowText(tt.total); got != tt.want {
				t.Errorf("reportRowText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReportSummaryText(t *testing.T) {
	t.Run("plural days", func(t *testing.T) {
		totals := []core.ProjectTotal{{Sessions: 2, Minutes: 40}, {Sessions: 1, Minutes: 25}}
		got := reportSummaryText(7, totals)
		want := "Last 7 days: 3 pomodoros (1h 5m)"
		if got != want {
			t.Errorf("reportSummaryText() = %q, want %q", got, want)
		}
	})

	t.Run("a single day uses the singular noun", func(t *testing.T) {
		got := reportSummaryText(1, nil)
		want := "Last 1 day: 0 pomodoros (0m)"
		if got != want {
			t.Errorf("reportSummaryText() = %q, want %q", got, want)
		}
	})
}

func TestNewReportTab(t *testing.T) {
	// A regression guard for the tab-building code path with an actual
	// session to report on. (The per-project rows live inside a
	// widget.List, whose items are not instantiated without a real
	// render — see reportRowText/reportSummaryText above for the
	// content logic those rows use, tested directly.)
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	project, err := store.projects.Add("Ship v1", "", core.Date{}, core.Today())
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	start := time.Now()
	store.sessions = core.Sessions{core.NewSession(start, start.Add(25*time.Minute), 25, nil, project)}

	env := &app.Env{Store: store, Now: func() time.Time { return time.Now() }}
	if content := NewReportTab(env); content == nil {
		t.Error("NewReportTab() = nil")
	}
}
