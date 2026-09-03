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

	tests := []struct {
		name string
		task *core.Task
		want string
	}{
		{"filed under a project", filed, "Write docs  ·  Ship v1  ·  1/4"},
		{"unfiled", unfiled, "Buy milk  ·  0/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := taskRowText(tt.task, store.projects); got != tt.want {
				t.Errorf("taskRowText() = %q, want %q", got, tt.want)
			}
		})
	}
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
