package core

import (
	"errors"
	"fmt"
	"time"
)

// ErrEmptyName is returned when a task, habit or project is given a
// name that is blank or only whitespace.
var ErrEmptyName = errors.New("core: name cannot be empty")

// ErrNotFound is returned when an id does not match any record.
//
// The Python original returned None from these lookups. Go's explicit
// error is better here: a caller that ignores it now fails a vet or
// lint check, whereas a silently discarded None did not.
var ErrNotFound = errors.New("core: not found")

// Task is one item on the TODO list.
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Done        bool      `json:"done"`
	Estimate    int       `json:"estimate"`  // planned pomodoros
	Completed   int       `json:"completed"` // pomodoros actually finished
	Created     Timestamp `json:"created"`
	CompletedAt *string   `json:"completed_at"`
	ProjectID   string    `json:"project_id"` // owning project, or empty
}

// Tasks is the ordered task list.
type Tasks []*Task

// NewTask builds a task. The estimate is floored at one pomodoro,
// since a task expected to take no work is not a task.
func NewTask(title string, estimate int, projectID string) (*Task, error) {
	clean, ok := cleanTitle(title)
	if !ok {
		return nil, fmt.Errorf("task: %w", ErrEmptyName)
	}
	if estimate < 1 {
		estimate = 1
	}
	return &Task{
		ID:        newID(),
		Title:     clean,
		Estimate:  estimate,
		Created:   Timestamp(time.Now()),
		ProjectID: projectID,
	}, nil
}

// normalize repairs a task read from disk so the rest of the program
// can assume every field is sane, whatever version wrote the file.
func (t *Task) normalize() {
	if t.ID == "" {
		t.ID = newID()
	}
	if t.Title == "" {
		t.Title = "Untitled"
	}
	if t.Estimate < 1 {
		t.Estimate = 1
	}
	if t.Completed < 0 {
		t.Completed = 0
	}
	if t.Created.Time().IsZero() {
		t.Created = Timestamp(time.Now())
	}
}

// ProgressLabel renders "completed/estimate" for the task table.
func (t *Task) ProgressLabel() string {
	return fmt.Sprintf("%d/%d", t.Completed, t.Estimate)
}

// Remaining reports the estimated pomodoros still outstanding, never
// negative even when the estimate has been overrun.
func (t *Task) Remaining() int {
	if t.Completed >= t.Estimate {
		return 0
	}
	return t.Estimate - t.Completed
}

// SetDone marks the task complete or reopens it, stamping or clearing
// the completion time to match.
func (t *Task) SetDone(done bool) {
	t.Done = done
	if done {
		stamp := Timestamp(time.Now()).String()
		t.CompletedAt = &stamp
	} else {
		t.CompletedAt = nil
	}
}

// Find returns the task with the given id.
func (ts Tasks) Find(id string) (*Task, error) {
	for _, t := range ts {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("task %q: %w", id, ErrNotFound)
}

// Add appends a new task and returns it.
func (ts *Tasks) Add(title string, estimate int, projectID string) (*Task, error) {
	task, err := NewTask(title, estimate, projectID)
	if err != nil {
		return nil, err
	}
	*ts = append(*ts, task)
	return task, nil
}

// Delete removes a task, reporting whether anything was removed.
//
// Sessions that referenced it keep their copy of the title, so history
// is never corrupted by a deletion.
func (ts *Tasks) Delete(id string) bool {
	for i, t := range *ts {
		if t.ID == id {
			*ts = append((*ts)[:i], (*ts)[i+1:]...)
			return true
		}
	}
	return false
}

// Move reorders a task by delta positions (-1 up, +1 down), clamping at
// the ends of the list rather than wrapping. Returns the new index.
func (ts Tasks) Move(id string, delta int) (int, error) {
	return move(ts, delta, func(i int) bool { return ts[i].ID == id },
		func(from, to int) {
			item := ts[from]
			copyShift(ts, from, to)
			ts[to] = item
		})
}

// ClearCompleted removes every finished task, reporting how many went.
func (ts *Tasks) ClearCompleted() int {
	kept := make(Tasks, 0, len(*ts))
	for _, t := range *ts {
		if !t.Done {
			kept = append(kept, t)
		}
	}
	removed := len(*ts) - len(kept)
	*ts = kept
	return removed
}

// Open returns the unfinished tasks, in list order.
func (ts Tasks) Open() Tasks {
	var out Tasks
	for _, t := range ts {
		if !t.Done {
			out = append(out, t)
		}
	}
	return out
}

// ForProject returns every task filed under a project.
func (ts Tasks) ForProject(projectID string) Tasks {
	var out Tasks
	for _, t := range ts {
		if t.ProjectID == projectID {
			out = append(out, t)
		}
	}
	return out
}

// Unfiled returns the tasks not belonging to any project.
func (ts Tasks) Unfiled() Tasks { return ts.ForProject("") }

// DetachFromProject unfiles every task belonging to a project, and
// reports how many moved.
//
// Deleting a project calls this rather than deleting its tasks:
// destroying someone's work because they closed a container would be a
// poor trade.
func (ts Tasks) DetachFromProject(projectID string) int {
	moved := 0
	for _, t := range ts {
		if t.ProjectID == projectID {
			t.ProjectID = ""
			moved++
		}
	}
	return moved
}

// TaskStats reports the open and finished counts plus the estimated
// pomodoros still outstanding across the open tasks.
func (ts Tasks) TaskStats() (open, done, remaining int) {
	for _, t := range ts {
		if t.Done {
			done++
			continue
		}
		open++
		remaining += t.Remaining()
	}
	return open, done, remaining
}
