package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// fakeStore is an in-memory app.Store for view tests.
type fakeStore struct {
	sessions core.Sessions
	tasks    core.Tasks
	habits   core.Habits
	projects core.Projects
}

func (f *fakeStore) Dir() string                        { return "/fake" }
func (f *fakeStore) LoadSessions() core.Sessions        { return f.sessions }
func (f *fakeStore) SaveSessions(s core.Sessions) error { f.sessions = s; return nil }
func (f *fakeStore) LoadTasks() core.Tasks              { return f.tasks }
func (f *fakeStore) SaveTasks(t core.Tasks) error       { f.tasks = t; return nil }
func (f *fakeStore) LoadHabits() core.Habits            { return f.habits }
func (f *fakeStore) SaveHabits(h core.Habits) error     { f.habits = h; return nil }
func (f *fakeStore) LoadProjects() core.Projects        { return f.projects }
func (f *fakeStore) SaveProjects(p core.Projects) error { f.projects = p; return nil }

// labelTexts walks a canvas object tree and collects every widget.Label's text.
func labelTexts(t *testing.T, o fyne.CanvasObject) []string {
	t.Helper()
	var out []string
	switch v := o.(type) {
	case *widget.Label:
		out = append(out, v.Text)
	case *fyne.Container:
		for _, child := range v.Objects {
			out = append(out, labelTexts(t, child)...)
		}
	}
	return out
}

func TestNewDashboard(t *testing.T) {
	today := core.NewDate(2026, time.September, 3)
	store := &fakeStore{}

	task, err := store.tasks.Add("Write docs", 4, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	task.Completed = 1

	habit, err := store.habits.Add("Review inbox", core.Daily, today)
	if err != nil {
		t.Fatalf("habits.Add() error = %v", err)
	}
	habit.Mark(today, true)

	if _, err := store.projects.Add("Ship v1", "", core.Date{}, today); err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}

	env := &app.Env{
		Store: store,
		Now:   func() time.Time { return time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC) },
	}

	content := NewDashboard(env)
	texts := labelTexts(t, content)
	joined := strings.Join(texts, "\n")

	for _, want := range []string{
		"Today: 0 pomodoros",
		"Tasks: 1 open, 0 done",
		"Habits: 1/1 kept today",
		"Projects: 1 active, 0 completed",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("dashboard text = %q, want substring %q", joined, want)
		}
	}
}

func TestNewDashboardFlagsOverdueTasks(t *testing.T) {
	store := &fakeStore{}

	task, err := store.tasks.Add("Overdue thing", 1, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	task.Due = core.NewDate(2026, time.August, 1)

	env := &app.Env{
		Store: store,
		Now:   func() time.Time { return time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC) },
	}

	joined := strings.Join(labelTexts(t, NewDashboard(env)), "\n")
	if !strings.Contains(joined, "1 overdue") {
		t.Errorf("dashboard text = %q, want it to flag the overdue task", joined)
	}
}
