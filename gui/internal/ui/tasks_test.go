package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

func TestTaskRowText(t *testing.T) {
	store := &fakeStore{}
	today := core.NewDate(2026, time.September, 3)
	project, err := store.projects.Add("Ship v1", "", core.Date{}, today)
	if err != nil {
		t.Fatalf("projects.Add() error = %v", err)
	}
	filed, err := store.tasks.Add("Write docs", 4, project.ID)
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	filed.Completed = 1
	unfiled, err := store.tasks.Add("Buy milk", 1, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}

	overdue, err := store.tasks.Add("File taxes", 1, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	overdue.Due = core.NewDate(2026, time.August, 1)

	future, err := store.tasks.Add("Renew license", 1, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	future.Due = core.NewDate(2026, time.December, 1)

	prioritized, err := store.tasks.Add("Ship it", 1, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	prioritized.Priority = core.PriorityHigh

	tagged, err := store.tasks.Add("Call plumber", 1, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	tagged.Tags = []string{"home", "urgent"}

	tests := []struct {
		name string
		task *core.Task
		want string
	}{
		{"filed under a project", filed, "Write docs  ·  Ship v1  ·  1/4"},
		{"unfiled", unfiled, "Buy milk  ·  0/1"},
		{"overdue", overdue, "File taxes  ·  0/1  ·  due 2026-08-01 (overdue)"},
		{"due in the future", future, "Renew license  ·  0/1  ·  due 2026-12-01"},
		{"a priority is shown", prioritized, "Ship it  ·  0/1  ·  high"},
		{"tags are shown", tagged, "Call plumber  ·  0/1  ·  home,urgent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskRowText(tt.task, store.projects, today); got != tt.want {
				t.Errorf("taskRowText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilteredTasks(t *testing.T) {
	var tasks core.Tasks
	none, _ := tasks.Add("None", 1, "")
	low, _ := tasks.Add("Low", 1, "")
	high1, _ := tasks.Add("High 1", 1, "")
	high2, _ := tasks.Add("High 2", 1, "")
	low.Priority = core.PriorityLow
	high1.Priority, high2.Priority = core.PriorityHigh, core.PriorityHigh
	high1.Tags = []string{"urgent"}
	_ = none

	names := func(ts core.Tasks) []string {
		out := make([]string, len(ts))
		for i, t := range ts {
			out[i] = t.Title
		}
		return out
	}
	equal := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	t.Run("All applies no priority filter", func(t *testing.T) {
		got := filteredTasks(tasks, "All", "")
		if !equal(names(got), []string{"None", "Low", "High 1", "High 2"}) {
			t.Errorf("got = %v", names(got))
		}
	})

	t.Run("None filters to tasks with no priority set", func(t *testing.T) {
		got := filteredTasks(tasks, "None", "")
		if !equal(names(got), []string{"None"}) {
			t.Errorf("got = %v", names(got))
		}
	})

	t.Run("a named level filters to exactly that priority", func(t *testing.T) {
		got := filteredTasks(tasks, "High", "")
		if !equal(names(got), []string{"High 1", "High 2"}) {
			t.Errorf("got = %v", names(got))
		}
	})

	t.Run("a tag filter narrows further", func(t *testing.T) {
		got := filteredTasks(tasks, "High", "urgent")
		if !equal(names(got), []string{"High 1"}) {
			t.Errorf("got = %v", names(got))
		}
	})

	t.Run("a tag filter alone ignores priority", func(t *testing.T) {
		got := filteredTasks(tasks, "All", "URGENT")
		if !equal(names(got), []string{"High 1"}) {
			t.Errorf("got = %v", names(got))
		}
	})
}

func TestNewTasksTab(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	if _, err := store.tasks.Add("Write docs", 4, ""); err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	env := &app.Env{Store: store, Now: func() time.Time { return time.Now() }}

	if content := NewTasksTab(env); content == nil {
		t.Error("NewTasksTab() = nil")
	}
}
