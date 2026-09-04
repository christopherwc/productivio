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

func TestFlattenProjectTree(t *testing.T) {
	var projects core.Projects
	website, err := projects.Add("Website", "", core.Date{}, core.Today())
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	redesign, err := projects.Add("Redesign", "", core.Date{}, core.Today())
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	other, err := projects.Add("Other", "", core.Date{}, core.Today())
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	if err := projects.SetParent(redesign.ID, website.ID); err != nil {
		t.Fatalf("SetParent() error = %v", err)
	}

	got := flattenProjectTree(projects)
	want := []projectEntry{
		{project: website, depth: 0},
		{project: redesign, depth: 1},
		{project: other, depth: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("flattenProjectTree() = %d entries, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].project != w.project || got[i].depth != w.depth {
			t.Errorf("entry %d = {%s depth=%d}, want {%s depth=%d}",
				i, got[i].project.Name, got[i].depth, w.project.Name, w.depth)
		}
	}
}

func TestNewProjectsTabWithSubprojects(t *testing.T) {
	// A regression guard for the tab-building code path that has
	// subprojects to flatten and indent: it must not panic when a
	// project's ParentID points at another entry in the same store.
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	website, err := store.projects.Add("Website", "", core.Date{}, core.Today())
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	redesign, err := store.projects.Add("Redesign", "", core.Date{}, core.Today())
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	if err := store.projects.SetParent(redesign.ID, website.ID); err != nil {
		t.Fatalf("SetParent() error = %v", err)
	}

	env := &app.Env{Store: store, Now: func() time.Time { return time.Now() }}
	if content := NewProjectsTab(env); content == nil {
		t.Error("NewProjectsTab() = nil")
	}
}
