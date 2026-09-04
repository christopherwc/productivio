package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

func TestHabitRowText(t *testing.T) {
	store := &fakeStore{}
	today := core.NewDate(2026, time.September, 3)

	habit, err := store.habits.Add("Review inbox", core.Daily, today)
	if err != nil {
		t.Fatalf("habits.Add() error = %v", err)
	}
	habit.Mark(today, true)

	got := habitRowText(habit, today)
	for _, want := range []string{"Review inbox", "Every day", "streak 1", "100% (30d)"} {
		if !strings.Contains(got, want) {
			t.Errorf("habitRowText() = %q, want substring %q", got, want)
		}
	}
}

func TestNewHabitsTab(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	store := &fakeStore{}
	if _, err := store.habits.Add("Review inbox", core.Daily, core.Today()); err != nil {
		t.Fatalf("habits.Add() error = %v", err)
	}
	env := &app.Env{Store: store, Now: func() time.Time { return time.Now() }}

	if content := NewHabitsTab(env); content == nil {
		t.Error("NewHabitsTab() = nil")
	}
}
