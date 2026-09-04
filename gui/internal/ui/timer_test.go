package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
)

func TestNewTimerTab(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	if _, err := store.tasks.Add("Write docs", 4, ""); err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	env := &app.Env{
		Store:     store,
		Now:       func() time.Time { return time.Now() },
		Notify:    func(string, string) bool { return true },
		AfterFunc: func(d time.Duration, f func()) *time.Timer { return time.AfterFunc(d, f) },
	}

	if content := NewTimerTab(env); content == nil {
		t.Error("NewTimerTab() = nil")
	}
}

func TestNewTimerTabWithNoOpenTasks(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	env := &app.Env{
		Store:  &fakeStore{},
		Now:    func() time.Time { return time.Now() },
		Notify: func(string, string) bool { return true },
	}

	if content := NewTimerTab(env); content == nil {
		t.Error("NewTimerTab() = nil")
	}
}
