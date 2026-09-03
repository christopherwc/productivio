package core

import "time"

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

func (ss Sessions) filter(keep func(*Session) bool) Sessions {
	var out Sessions
	for _, s := range ss {
		if keep(s) {
			out = append(out, s)
		}
	}
	return out
}
