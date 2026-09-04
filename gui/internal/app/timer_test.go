package app

import (
	"testing"
	"time"

	"github.com/christopherwc/productivio/internal/core"
)

// fakeTimerStore is an in-memory Store for timer tests.
type fakeTimerStore struct {
	sessions core.Sessions
	tasks    core.Tasks
	habits   core.Habits
	projects core.Projects
}

func (f *fakeTimerStore) Dir() string                        { return "/fake" }
func (f *fakeTimerStore) LoadSessions() core.Sessions        { return f.sessions }
func (f *fakeTimerStore) SaveSessions(s core.Sessions) error { f.sessions = s; return nil }
func (f *fakeTimerStore) LoadTasks() core.Tasks              { return f.tasks }
func (f *fakeTimerStore) SaveTasks(t core.Tasks) error       { f.tasks = t; return nil }
func (f *fakeTimerStore) LoadHabits() core.Habits            { return f.habits }
func (f *fakeTimerStore) SaveHabits(h core.Habits) error     { f.habits = h; return nil }
func (f *fakeTimerStore) LoadProjects() core.Projects        { return f.projects }
func (f *fakeTimerStore) SaveProjects(p core.Projects) error { f.projects = p; return nil }

func testEnv(store Store, now time.Time) *Env {
	notified := 0
	return &Env{
		Store:  store,
		Now:    func() time.Time { return now },
		Notify: func(string, string) bool { notified++; return true },
	}
}

func TestTimerWorkToRestToIdle(t *testing.T) {
	env := testEnv(&fakeTimerStore{}, time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC))
	timer := NewTimer(env)

	timer.Start(2*time.Minute, 1*time.Minute, "")
	if timer.Phase != PhaseWork {
		t.Fatalf("Phase = %v, want PhaseWork", timer.Phase)
	}

	for i := 0; i < 119; i++ {
		timer.Tick()
	}
	if timer.Phase != PhaseWork || timer.Remaining != time.Second {
		t.Fatalf("after 119 ticks: Phase = %v, Remaining = %v, want PhaseWork/1s", timer.Phase, timer.Remaining)
	}

	timer.Tick() // the 120th tick: work completes, rest begins
	if timer.Phase != PhaseRest || timer.Remaining != time.Minute {
		t.Fatalf("after work completes: Phase = %v, Remaining = %v, want PhaseRest/1m", timer.Phase, timer.Remaining)
	}

	for i := 0; i < 60; i++ {
		timer.Tick()
	}
	if timer.Phase != PhaseIdle || timer.Remaining != 0 {
		t.Fatalf("after rest completes: Phase = %v, Remaining = %v, want PhaseIdle/0", timer.Phase, timer.Remaining)
	}

	// Idle: further ticks must not panic or go negative.
	timer.Tick()
	if timer.Phase != PhaseIdle || timer.Remaining != 0 {
		t.Errorf("tick while idle changed state: Phase = %v, Remaining = %v", timer.Phase, timer.Remaining)
	}
}

func TestTimerPause(t *testing.T) {
	env := testEnv(&fakeTimerStore{}, time.Now())
	timer := NewTimer(env)
	timer.Start(time.Minute, time.Minute, "")

	timer.TogglePause()
	if !timer.Paused {
		t.Fatal("TogglePause() did not pause a running timer")
	}
	before := timer.Remaining
	timer.Tick()
	timer.Tick()
	if timer.Remaining != before {
		t.Errorf("Remaining changed while paused: got %v, want %v", timer.Remaining, before)
	}

	timer.TogglePause()
	timer.Tick()
	if timer.Remaining != before-time.Second {
		t.Errorf("Remaining after resume+tick = %v, want %v", timer.Remaining, before-time.Second)
	}
}

func TestTimerTogglePauseWhileIdle(t *testing.T) {
	env := testEnv(&fakeTimerStore{}, time.Now())
	timer := NewTimer(env)

	timer.TogglePause()
	if timer.Paused {
		t.Error("TogglePause() while idle should be a no-op")
	}
}

func TestTimerCompletesWorkCreditsTaskAndSession(t *testing.T) {
	store := &fakeTimerStore{}
	task, err := store.tasks.Add("Write docs", 4, "")
	if err != nil {
		t.Fatalf("tasks.Add() error = %v", err)
	}
	start := time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)
	env := testEnv(store, start)
	timer := NewTimer(env)

	timer.Start(time.Minute, time.Minute, task.ID)
	for i := 0; i < 60; i++ {
		timer.Tick()
	}

	if got := store.tasks[0].Completed; got != 1 {
		t.Errorf("task.Completed = %d, want 1", got)
	}
	if len(store.sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(store.sessions))
	}
	session := store.sessions[0]
	if session.TaskID != task.ID || session.WorkMinutes != 1 {
		t.Errorf("session = %+v, want TaskID=%q WorkMinutes=1", session, task.ID)
	}
}

func TestTimerCompletesWorkWithDeletedTask(t *testing.T) {
	// The task credited at Start may be gone by the time the work
	// phase completes — deleted from another tab while the timer ran.
	// completeWork must not panic, and should still record the
	// session (as unattributed work) rather than losing the interval.
	env := testEnv(&fakeTimerStore{}, time.Now())
	timer := NewTimer(env)

	timer.Start(time.Minute, time.Minute, "some-id-that-was-never-added")
	for i := 0; i < 60; i++ {
		timer.Tick()
	}

	store := env.Store.(*fakeTimerStore)
	if len(store.sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1", len(store.sessions))
	}
	if store.sessions[0].TaskID != "" {
		t.Errorf("TaskID = %q, want empty (task not found)", store.sessions[0].TaskID)
	}
}
