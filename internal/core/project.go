package core

import "fmt"

// Project statuses.
const (
	StatusActive    = "active"
	StatusOnHold    = "on hold"
	StatusCompleted = "completed"
)

// ProjectStatuses is the set a project may hold.
var ProjectStatuses = []string{StatusActive, StatusOnHold, StatusCompleted}

// Health labels, in the spirit of Todoist's project health signal but
// computed from deterministic rules (see Health) rather than from
// opaque activity heuristics, so the label is reproducible and
// unit-testable.
const (
	HealthCompleted  = "Completed"
	HealthOnHold     = "On hold"
	HealthOverdue    = "Overdue"
	HealthAtRisk     = "At risk"
	HealthNotStarted = "Not started"
	HealthOnTrack    = "On track"
)

// AtRiskSlip is how far actual progress may lag the elapsed share of a
// project's timeline before it is flagged "At risk". A project 60%
// through its schedule with 30% of tasks done is 0.30 behind, and trips
// this.
const AtRiskSlip = 0.25

// Project is a goal made of many tasks. It is the layer that ties the
// rest of the application together: tasks belong to a project, and
// because every completed pomodoro already records which task it
// served, focus time rolls up into project-level reporting.
type Project struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Created     Date    `json:"created"`
	Due         Date    `json:"due"`
	CompletedAt *string `json:"completed_at"`
}

// Projects is the ordered project list.
type Projects []*Project

// NewProject builds a project. due may be the zero Date for no
// deadline.
func NewProject(name, description string, due, created Date) (*Project, error) {
	clean, ok := cleanTitle(name)
	if !ok {
		return nil, fmt.Errorf("project: %w", ErrEmptyName)
	}
	if created.IsZero() {
		created = Today()
	}
	return &Project{
		ID:          newID(),
		Name:        clean,
		Description: description,
		Status:      StatusActive,
		Created:     created,
		Due:         due,
	}, nil
}

// normalize repairs a project read from disk. An unrecognised status
// becomes "active", so a hand-edited file can never leave a project in
// a state the UI cannot render.
func (p *Project) normalize() {
	if p.ID == "" {
		p.ID = newID()
	}
	if p.Name == "" {
		p.Name = "Untitled"
	}
	if !validStatus(p.Status) {
		p.Status = StatusActive
	}
	if p.Created.IsZero() {
		p.Created = Today()
	}
}

func validStatus(status string) bool {
	for _, s := range ProjectStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// SetStatus changes the status, stamping or clearing the completion
// date. An unrecognised status is rejected rather than stored.
//
// Use this for a status that came from outside the program. For the
// known-good constants, prefer Complete, Hold or Reopen: they cannot
// fail, so callers are not left writing an error branch that can never
// be taken and can never be tested.
func (p *Project) SetStatus(status string) error {
	if !validStatus(status) {
		return fmt.Errorf("project: unknown status %q", status)
	}
	p.setStatus(status)
	return nil
}

// Complete marks the project finished.
func (p *Project) Complete() { p.setStatus(StatusCompleted) }

// Hold pauses the project.
func (p *Project) Hold() { p.setStatus(StatusOnHold) }

// Reopen returns the project to active.
func (p *Project) Reopen() { p.setStatus(StatusActive) }

func (p *Project) setStatus(status string) {
	p.Status = status
	if status == StatusCompleted {
		stamp := Today().String()
		p.CompletedAt = &stamp
	} else {
		p.CompletedAt = nil
	}
}

// IsOverdue reports whether the deadline has passed with work still
// outstanding. A completed project is never overdue.
func (p *Project) IsOverdue(today Date) bool {
	if p.Status == StatusCompleted || p.Due.IsZero() {
		return false
	}
	return p.Due.Before(today)
}

// DaysUntilDue counts days to the deadline, negative when overdue. The
// second result is false when there is no deadline, since zero days is
// a meaningful value that must not be confused with "unset".
func (p *Project) DaysUntilDue(today Date) (int, bool) {
	if p.Due.IsZero() {
		return 0, false
	}
	return today.DaysUntil(p.Due), true
}

// ScheduleElapsed reports how much of the created-to-due window has
// passed, from 0.0 to 1.0. The second result is false when there is no
// deadline, since without one there is nothing to be behind schedule
// against. A window that starts and ends on the same day counts as
// fully elapsed.
func (p *Project) ScheduleElapsed(today Date) (float64, bool) {
	if p.Due.IsZero() {
		return 0, false
	}
	span := p.Created.DaysUntil(p.Due)
	if span <= 0 {
		return 1, true
	}
	elapsed := float64(p.Created.DaysUntil(today)) / float64(span)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > 1 {
		elapsed = 1
	}
	return elapsed, true
}

// TaskProgress reports completed and total tasks and the fraction done.
//
// The fraction is zero for a project with no tasks, so callers never
// have to guard against dividing by zero. Note that adding tasks lowers
// this figure even though nothing was undone: the scope grew.
func (ps Projects) TaskProgress(tasks Tasks, projectID string) (done, total int, fraction float64) {
	owned := tasks.ForProject(projectID)
	for _, t := range owned {
		if t.Done {
			done++
		}
	}
	total = len(owned)
	if total == 0 {
		return 0, 0, 0
	}
	return done, total, float64(done) / float64(total)
}

// EffortProgress reports completed and estimated pomodoros and the
// fraction spent.
//
// The fraction is deliberately not capped at 1.0: going over the
// estimate is exactly the signal worth surfacing.
func (ps Projects) EffortProgress(tasks Tasks, projectID string) (completed, estimated int, fraction float64) {
	for _, t := range tasks.ForProject(projectID) {
		completed += t.Completed
		estimated += t.Estimate
	}
	if estimated == 0 {
		return completed, 0, 0
	}
	return completed, estimated, float64(completed) / float64(estimated)
}

// Health returns a short status label from deterministic rules,
// evaluated in order with the first match winning:
//
//  1. Completed    the user marked it done.
//  2. On hold      the user paused it.
//  3. Overdue      the deadline has passed with work outstanding.
//  4. At risk      task progress lags the elapsed share of the
//     schedule by more than AtRiskSlip.
//  5. Not started  no tasks finished and no pomodoros logged.
//  6. On track     everything else.
func (ps Projects) Health(p *Project, tasks Tasks, today Date) string {
	switch {
	case p.Status == StatusCompleted:
		return HealthCompleted
	case p.Status == StatusOnHold:
		return HealthOnHold
	case p.IsOverdue(today):
		return HealthOverdue
	}

	done, _, fraction := ps.TaskProgress(tasks, p.ID)
	if elapsed, hasDeadline := p.ScheduleElapsed(today); hasDeadline && elapsed-fraction > AtRiskSlip {
		return HealthAtRisk
	}

	completed, _, _ := ps.EffortProgress(tasks, p.ID)
	if done == 0 && completed == 0 {
		return HealthNotStarted
	}
	return HealthOnTrack
}

// Summary is everything the UI needs about one project, gathered in one
// value so the interface layer does not recompute the same rollups in
// three places and so the whole thing can be asserted in one test.
type Summary struct {
	ID                 string
	Name               string
	Status             string
	Health             string
	TasksDone          int
	TasksTotal         int
	TaskFraction       float64
	PomodorosDone      int
	PomodorosEstimated int
	EffortFraction     float64
	Minutes            int
	SessionCount       int
	Due                Date
	DaysLeft           int
	HasDeadline        bool
	Overdue            bool
}

// Summarize gathers every rollup for one project.
func (ps Projects) Summarize(p *Project, tasks Tasks, sessions Sessions, today Date) Summary {
	done, total, taskFraction := ps.TaskProgress(tasks, p.ID)
	completed, estimated, effortFraction := ps.EffortProgress(tasks, p.ID)
	daysLeft, hasDeadline := p.DaysUntilDue(today)
	return Summary{
		ID:                 p.ID,
		Name:               p.Name,
		Status:             p.Status,
		Health:             ps.Health(p, tasks, today),
		TasksDone:          done,
		TasksTotal:         total,
		TaskFraction:       taskFraction,
		PomodorosDone:      completed,
		PomodorosEstimated: estimated,
		EffortFraction:     effortFraction,
		Minutes:            sessions.ProjectMinutes(p.ID),
		SessionCount:       len(sessions.ForProject(p.ID)),
		Due:                p.Due,
		DaysLeft:           daysLeft,
		HasDeadline:        hasDeadline,
		Overdue:            p.IsOverdue(today),
	}
}

// Find returns the project with the given id.
func (ps Projects) Find(id string) (*Project, error) {
	for _, p := range ps {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project %q: %w", id, ErrNotFound)
}

// NameOf returns a project's display name, or fallback when the id
// matches nothing. The task table uses this, where most rows have no
// project and an em dash reads better than an empty cell.
func (ps Projects) NameOf(id, fallback string) string {
	if p, err := ps.Find(id); err == nil {
		return p.Name
	}
	return fallback
}

// Add appends a new project and returns it.
func (ps *Projects) Add(name, description string, due, created Date) (*Project, error) {
	project, err := NewProject(name, description, due, created)
	if err != nil {
		return nil, err
	}
	*ps = append(*ps, project)
	return project, nil
}

// Delete removes a project, reporting whether it existed.
//
// Its tasks are not deleted; see Tasks.DetachFromProject.
func (ps *Projects) Delete(id string) bool {
	for i, p := range *ps {
		if p.ID == id {
			*ps = append((*ps)[:i], (*ps)[i+1:]...)
			return true
		}
	}
	return false
}

// Move reorders a project by delta positions, clamping at the ends.
func (ps Projects) Move(id string, delta int) (int, error) {
	return move(ps, delta, func(i int) bool { return ps[i].ID == id },
		func(from, to int) {
			item := ps[from]
			copyShift(ps, from, to)
			ps[to] = item
		})
}

// Active returns the projects that are neither completed nor on hold.
func (ps Projects) Active() Projects {
	var out Projects
	for _, p := range ps {
		if p.Status == StatusActive {
			out = append(out, p)
		}
	}
	return out
}

// PortfolioStats reports the active and completed counts plus how many
// projects need attention (overdue or at risk).
func (ps Projects) PortfolioStats(tasks Tasks, today Date) (active, completed, needAttention int) {
	for _, p := range ps {
		if p.Status == StatusCompleted {
			completed++
			continue
		}
		if p.Status == StatusActive {
			active++
		}
		switch ps.Health(p, tasks, today) {
		case HealthOverdue, HealthAtRisk:
			needAttention++
		}
	}
	return active, completed, needAttention
}
