package app

import (
	"time"

	"github.com/christopherwc/productivio/internal/core"
)

// TimerPhase is which part of a pomodoro cycle a Timer is in.
type TimerPhase int

const (
	PhaseIdle TimerPhase = iota
	PhaseWork
	PhaseRest
)

// Timer drives one pomodoro work/rest cycle.
//
// Tick is a plain, synchronous method — Timer starts no goroutines of
// its own and has no Fyne dependency, which is what makes it directly
// unit-testable with a fake clock. The caller is responsible for
// invoking Tick from wherever is safe to touch env.Store and
// env.Notify; the GUI always does that inside fyne.Do (see
// internal/ui/timer.go's scheduling loop), never from the raw
// env.AfterFunc callback itself.
type Timer struct {
	env *Env

	Phase     TimerPhase
	Remaining time.Duration
	Paused    bool

	work, rest time.Duration
	taskID     string
	started    time.Time
}

// NewTimer returns an idle Timer bound to env.
func NewTimer(env *Env) *Timer {
	return &Timer{env: env}
}

// Start begins a work interval, optionally credited to a task by id
// (and, transitively, its project — see completeWork). Starting while
// already running restarts the cycle.
func (t *Timer) Start(work, rest time.Duration, taskID string) {
	t.work, t.rest = work, rest
	t.taskID = taskID
	t.Phase = PhaseWork
	t.Remaining = work
	t.Paused = false
	t.started = t.env.Now()
}

// TogglePause pauses a running timer, or resumes a paused one. A no-op
// while idle.
func (t *Timer) TogglePause() {
	if t.Phase != PhaseIdle {
		t.Paused = !t.Paused
	}
}

// Tick advances the timer by one second. A no-op while idle or
// paused. Handles the work→rest→idle transition, including session
// persistence and task credit when a work phase completes, and a
// desktop notification at every transition — the same behavior
// internal/cli's `pomodoro start` countdown has.
func (t *Timer) Tick() {
	if t.Phase == PhaseIdle || t.Paused || t.Remaining <= 0 {
		return
	}
	t.Remaining -= time.Second
	if t.Remaining > 0 {
		return
	}

	switch t.Phase {
	case PhaseWork:
		t.completeWork()
		t.Phase = PhaseRest
		t.Remaining = t.rest
		t.env.Notify("Pomodoro complete", "Time for a break.")
	case PhaseRest:
		t.Phase = PhaseIdle
		t.Remaining = 0
		t.env.Notify("Break over", "Back to work.")
	}
}

// completeWork records the finished interval as a session and, if it
// was credited to a task, increments that task's completed count.
// Both are looked up fresh by id rather than trusting a pointer
// captured at Start: the task or project may have been edited or
// deleted from another tab while the timer was running.
func (t *Timer) completeWork() {
	tasks := t.env.Store.LoadTasks()

	var task *core.Task
	if t.taskID != "" {
		if found, err := tasks.Find(t.taskID); err == nil {
			task = found
		}
	}

	var project *core.Project
	if task != nil && task.ProjectID != "" {
		if found, err := t.env.Store.LoadProjects().Find(task.ProjectID); err == nil {
			project = found
		}
	}

	end := t.env.Now()
	sessions := append(t.env.Store.LoadSessions(),
		core.NewSession(t.started, end, int(t.work/time.Minute), task, project))
	_ = t.env.Store.SaveSessions(sessions)

	if task != nil {
		task.Completed++
		_ = t.env.Store.SaveTasks(tasks)
	}
}
