package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
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

// Priority is how urgently a task needs attention. The zero value,
// PriorityNone, sorts and displays as "no priority set" rather than
// "lowest", so an unprioritized task is never confused with one
// deliberately marked low.
type Priority int

const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
)

// ParsePriority reads a priority level by name, case-insensitively.
// "-" and "" both mean PriorityNone, matching the "-" placeholder the
// CLI already uses elsewhere to skip an optional field.
func ParsePriority(s string) (Priority, error) {
	switch strings.ToLower(s) {
	case "", "-", "none":
		return PriorityNone, nil
	case "low":
		return PriorityLow, nil
	case "medium", "med":
		return PriorityMedium, nil
	case "high":
		return PriorityHigh, nil
	default:
		return PriorityNone, fmt.Errorf("core: priority must be low, medium or high, got %q", s)
	}
}

// String renders the priority the way the CLI displays it.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	default:
		return "-"
	}
}

// name is the priority's persisted and parsed form ("" for none, so an
// old task file without a priority field still decodes cleanly).
func (p Priority) name() string {
	if p == PriorityNone {
		return ""
	}
	return p.String()
}

// MarshalJSON renders the priority by name, so the data file stays
// hand-editable like the rest of the model.
func (p Priority) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.name())
}

// UnmarshalJSON accepts a priority name. An unrecognized value decodes
// to PriorityNone rather than erroring, so a hand-edited or
// hand-written file degrades to "unset" instead of failing to load.
func (p *Priority) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParsePriority(s)
	if err != nil {
		parsed = PriorityNone
	}
	*p = parsed
	return nil
}

// Task is one item on the TODO list.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Done        bool       `json:"done"`
	Estimate    int        `json:"estimate"`  // planned pomodoros
	Completed   int        `json:"completed"` // pomodoros actually finished
	Created     Timestamp  `json:"created"`
	CompletedAt *string    `json:"completed_at"`
	ProjectID   string     `json:"project_id"`           // owning project, or empty
	Due         Date       `json:"due"`                  // deadline, or the zero Date for none
	Priority    Priority   `json:"priority"`             // urgency, or PriorityNone for unset
	Tags        []string   `json:"tags"`                 // freeform labels, never nil
	UpdatedAt   time.Time  `json:"updated_at"`           // last content change; Store.SaveTasks stamps this
	DeletedAt   *time.Time `json:"deleted_at,omitempty"` // tombstone; nil means live. See Tasks.Delete.
}

// ParseTags splits a comma-separated list into cleaned tags. "" and
// "-" (the CLI's placeholder for "skip this field") both mean no tags.
func ParseTags(s string) []string {
	if s == "" || s == "-" {
		return nil
	}
	return cleanTags(strings.Split(s, ","))
}

// cleanTags trims each tag, drops empties, and deduplicates
// case-insensitively while keeping the first casing seen.
func cleanTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tag)
	}
	return out
}

// FormatTags renders tags as a comma-separated list for display, or a
// dash when there are none.
func FormatTags(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	return strings.Join(tags, ",")
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
		Tags:      []string{},
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
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.Created.Time()
	}
	t.Tags = cleanTags(t.Tags)
	if t.Tags == nil {
		t.Tags = []string{}
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

// IsOverdue reports whether the deadline has passed with the task
// still open. A done task, or one with no due date, is never overdue —
// mirroring Project.IsOverdue.
func (t *Task) IsOverdue(today Date) bool {
	if t.Done || t.Due.IsZero() {
		return false
	}
	return t.Due.Before(today)
}

// DaysUntilDue counts days to the deadline, negative when overdue. The
// second result is false when there is no deadline, since zero days is
// a meaningful value that must not be confused with "unset".
func (t *Task) DaysUntilDue(today Date) (int, bool) {
	if t.Due.IsZero() {
		return 0, false
	}
	return today.DaysUntil(t.Due), true
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

// HasTag reports whether the task carries a tag, compared
// case-insensitively so "Urgent" and "urgent" are the same tag.
func (t *Task) HasTag(tag string) bool {
	target := strings.ToLower(strings.TrimSpace(tag))
	for _, tg := range t.Tags {
		if strings.ToLower(tg) == target {
			return true
		}
	}
	return false
}

// Find returns the task with the given id. A tombstoned task is treated
// as not found, matching how deletion behaved before tombstones existed.
func (ts Tasks) Find(id string) (*Task, error) {
	for _, t := range ts {
		if t.ID == id && t.DeletedAt == nil {
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

// Delete tombstones a task, reporting whether anything was removed. It
// stays in the slice with DeletedAt set rather than being spliced out,
// so a sync can propagate the deletion to another device instead of
// silently resurrecting the task on the next merge; every listing
// method below hides tombstoned tasks so this is invisible to a caller
// that never syncs.
//
// Sessions that referenced it keep their copy of the title, so history
// is never corrupted by a deletion.
func (ts *Tasks) Delete(id string) bool {
	for _, t := range *ts {
		if t.ID == id && t.DeletedAt == nil {
			now := time.Now()
			t.DeletedAt = &now
			return true
		}
	}
	return false
}

// Move reorders a task by delta positions (-1 up, +1 down), clamping at
// the ends of the list rather than wrapping. Returns the new index.
func (ts Tasks) Move(id string, delta int) (int, error) {
	return move(ts, delta, func(i int) bool { return ts[i].ID == id && ts[i].DeletedAt == nil },
		func(from, to int) {
			item := ts[from]
			copyShift(ts, from, to)
			ts[to] = item
		})
}

// ClearCompleted tombstones every finished task, reporting how many
// went. Like Delete, this marks rather than splices, for the same
// sync-propagation reason.
func (ts *Tasks) ClearCompleted() int {
	removed := 0
	for _, t := range *ts {
		if t.Done && t.DeletedAt == nil {
			now := time.Now()
			t.DeletedAt = &now
			removed++
		}
	}
	return removed
}

// Open returns the unfinished, non-deleted tasks, in list order.
func (ts Tasks) Open() Tasks {
	var out Tasks
	for _, t := range ts {
		if !t.Done && t.DeletedAt == nil {
			out = append(out, t)
		}
	}
	return out
}

// ForProject returns every non-deleted task filed under a project.
func (ts Tasks) ForProject(projectID string) Tasks {
	var out Tasks
	for _, t := range ts {
		if t.ProjectID == projectID && t.DeletedAt == nil {
			out = append(out, t)
		}
	}
	return out
}

// Unfiled returns the tasks not belonging to any project.
func (ts Tasks) Unfiled() Tasks { return ts.ForProject("") }

// WithPriority returns the non-deleted tasks at exactly the given
// priority level, in list order.
func (ts Tasks) WithPriority(p Priority) Tasks {
	var out Tasks
	for _, t := range ts {
		if t.Priority == p && t.DeletedAt == nil {
			out = append(out, t)
		}
	}
	return out
}

// Search returns the non-deleted tasks whose title contains the query,
// matched case-insensitively, in list order. A blank query matches
// nothing, since "every task" is what Open or the unfiltered list is
// for.
func (ts Tasks) Search(query string) Tasks {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out Tasks
	for _, t := range ts {
		if t.DeletedAt == nil && strings.Contains(strings.ToLower(t.Title), q) {
			out = append(out, t)
		}
	}
	return out
}

// WithTag returns the non-deleted tasks carrying a tag, in list order.
func (ts Tasks) WithTag(tag string) Tasks {
	var out Tasks
	for _, t := range ts {
		if t.DeletedAt == nil && t.HasTag(tag) {
			out = append(out, t)
		}
	}
	return out
}

// ByPriority returns the non-deleted tasks ordered highest priority
// first. Equal priorities keep their relative list order, so `task
// list` stays predictable rather than shuffling tasks that tie.
func (ts Tasks) ByPriority() Tasks {
	out := make(Tasks, 0, len(ts))
	for _, t := range ts {
		if t.DeletedAt == nil {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	return out
}

// DetachFromProject unfiles every task belonging to a project, and
// reports how many moved.
//
// Deleting a project calls this rather than deleting its tasks:
// destroying someone's work because they closed a container would be a
// poor trade.
func (ts Tasks) DetachFromProject(projectID string) int {
	moved := 0
	for _, t := range ts {
		if t.ProjectID == projectID && t.DeletedAt == nil {
			t.ProjectID = ""
			moved++
		}
	}
	return moved
}

// TaskStats reports the open and finished counts, the estimated
// pomodoros still outstanding across the open tasks, and how many open
// tasks are overdue. Tombstoned tasks count toward none of these.
func (ts Tasks) TaskStats(today Date) (open, done, remaining, overdue int) {
	for _, t := range ts {
		if t.DeletedAt != nil {
			continue
		}
		if t.Done {
			done++
			continue
		}
		open++
		remaining += t.Remaining()
		if t.IsOverdue(today) {
			overdue++
		}
	}
	return open, done, remaining, overdue
}
