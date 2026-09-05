package core

import (
	"sort"
	"time"
)

// Session is one completed work phase, the permanent record of a
// pomodoro.
//
// Task and project names are stored alongside their ids on purpose:
// history must stay readable after either is deleted, and it is what
// lets focus time roll up into project-level reporting rather than
// being generic minutes.
type Session struct {
	Start       Timestamp `json:"start"`
	End         Timestamp `json:"end"`
	WorkMinutes int       `json:"work_minutes"`
	TaskID      string    `json:"task_id"`
	TaskTitle   string    `json:"task_title"`
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
}

// Sessions is the full history, oldest first.
type Sessions []*Session

// NewSession builds one session record.
//
// task and project may both be nil, which is how an unattributed
// pomodoro is recorded.
func NewSession(start, end time.Time, workMinutes int, task *Task, project *Project) *Session {
	s := &Session{
		Start:       Timestamp(start),
		End:         Timestamp(end),
		WorkMinutes: workMinutes,
	}
	if task != nil {
		s.TaskID, s.TaskTitle = task.ID, task.Title
	}
	if project != nil {
		s.ProjectID, s.ProjectName = project.ID, project.Name
	}
	return s
}

// Day is the calendar day the session started on.
func (s *Session) Day() Date { return DateOf(s.Start.Time()) }

// DailySummary reports how many pomodoros were completed on a given day
// and how many minutes they totalled.
func (ss Sessions) DailySummary(day Date) (count, minutes int) {
	for _, s := range ss {
		if s.Day() == day {
			count++
			minutes += s.WorkMinutes
		}
	}
	return count, minutes
}

// ForTask returns every session worked against a task.
func (ss Sessions) ForTask(taskID string) Sessions {
	return ss.filter(func(s *Session) bool { return s.TaskID == taskID })
}

// ForProject returns every session that counted towards a project.
func (ss Sessions) ForProject(projectID string) Sessions {
	return ss.filter(func(s *Session) bool { return s.ProjectID == projectID })
}

// ProjectMinutes totals the focused minutes logged against a project.
func (ss Sessions) ProjectMinutes(projectID string) int {
	total := 0
	for _, s := range ss.ForProject(projectID) {
		total += s.WorkMinutes
	}
	return total
}

// ProjectTotal is one project's rolled-up focus time within a report
// window. ProjectName is "" for unattributed work — sessions with no
// project credited.
//
// The name comes from the session records themselves, not a Projects
// lookup, so a report still shows correctly under a project's old name
// even after it has been renamed or deleted; that is exactly why
// Session stores its own copy of the name rather than only the id.
type ProjectTotal struct {
	ProjectID   string
	ProjectName string
	Sessions    int
	Minutes     int
}

// Report totals focused time per project across every session on or
// after since, ordered by minutes descending (ties broken by name) so
// the projects worked on most show first.
func (ss Sessions) Report(since Date) []ProjectTotal {
	totals := map[string]*ProjectTotal{}
	var order []string
	for _, s := range ss {
		if s.Day().Before(since) {
			continue
		}
		key := s.ProjectID // "" is a valid key: unattributed work
		t, ok := totals[key]
		if !ok {
			t = &ProjectTotal{ProjectID: s.ProjectID, ProjectName: s.ProjectName}
			totals[key] = t
			order = append(order, key)
		}
		t.Sessions++
		t.Minutes += s.WorkMinutes
	}

	out := make([]ProjectTotal, 0, len(order))
	for _, key := range order {
		out = append(out, *totals[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Minutes != out[j].Minutes {
			return out[i].Minutes > out[j].Minutes
		}
		return out[i].ProjectName < out[j].ProjectName
	})
	return out
}

func (ss Sessions) filter(keep func(*Session) bool) Sessions {
	var out Sessions
	for _, s := range ss {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}
