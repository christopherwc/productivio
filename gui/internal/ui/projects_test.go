package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

func TestHealthColor(t *testing.T) {
	// Every health label must map to something, and overdue/at-risk
	// must not be confused with on-track: that would defeat the whole
	// point of coloring the label.
	seen := map[string]bool{}
	for _, health := range []string{
		core.HealthCompleted, core.HealthOnHold, core.HealthOverdue,
		core.HealthAtRisk, core.HealthNotStarted, core.HealthOnTrack,
	} {
		c := healthColor(health)
		if c == nil {
			t.Errorf("healthColor(%q) = nil", health)
		}
		seen[health] = true
	}
	if healthColor(core.HealthOverdue) == healthColor(core.HealthOnTrack) {
		t.Error("Overdue and On track must not share a color")
	}
}

func TestProjectRowText(t *testing.T) {
	store := &fakeStore{}
	today := core.NewDate(2026, time.September, 3)
	due := core.NewDate(2026, time.September, 30)

	project, err := store.projects.Add("Ship v1", "", due, today)
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	task, err := store.tasks.Add("Write docs", 4, project.ID)
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	task.Done = true

	got := projectRowText(project, store.projects, store.tasks, store.sessions, today)
	want := "Ship v1  ·  1/1 tasks  ·  due 2026-09-30"
	if got != want {
		t.Errorf("projectRowText() = %q, want %q", got, want)
	}
}

func TestNewProjectsTab(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	if _, err := store.projects.Add("Ship v1", "", core.Date{}, core.Today()); err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}

	env := &app.Env{Store: store, Now: func() time.Time { return time.Now() }}
	if content := NewProjectsTab(env); content == nil {
		t.Error("NewProjectsTab() = nil")
	}
}
