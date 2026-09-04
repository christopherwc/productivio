package core

import (
	"errors"
	"testing"
	"time"
)

func TestNewProject(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		p, err := NewProject("  Website  ", "Launch", Date{}, fixedToday)
		if err != nil {
			t.Fatal(err)
		}
		if p.Name != "Website" || p.Description != "Launch" {
			t.Errorf("fields = %+v", p)
		}
		if p.Status != StatusActive {
			t.Errorf("Status = %q, want active", p.Status)
		}
		if !p.Due.IsZero() {
			t.Error("a project with no deadline should have the zero date")
		}
	})

	t.Run("rejects a blank name", func(t *testing.T) {
		if _, err := NewProject("   ", "", Date{}, fixedToday); !errors.Is(err, ErrEmptyName) {
			t.Errorf("error = %v, want ErrEmptyName", err)
		}
	})

	t.Run("an unset creation date becomes today", func(t *testing.T) {
		p, err := NewProject("x", "", Date{}, Date{})
		if err != nil {
			t.Fatal(err)
		}
		if p.Created != Today() {
			t.Errorf("Created = %v, want today", p.Created)
		}
	})
}

func TestProjectNormalize(t *testing.T) {
	t.Run("fills gaps", func(t *testing.T) {
		p := &Project{ID: "abc"}
		p.normalize()
		if p.Name != "Untitled" || p.Status != StatusActive || p.Created.IsZero() {
			t.Errorf("normalize left gaps: %+v", p)
		}
	})

	t.Run("an unknown status becomes active", func(t *testing.T) {
		// A bad status must never reach the UI as an unrenderable value.
		p := &Project{ID: "a", Name: "x", Status: "banana"}
		p.normalize()
		if p.Status != StatusActive {
			t.Errorf("Status = %q, want active", p.Status)
		}
	})

	t.Run("generates a missing id", func(t *testing.T) {
		p := &Project{}
		p.normalize()
		if p.ID == "" {
			t.Error("normalize should supply an id")
		}
	})
}

func TestProjectStatus(t *testing.T) {
	p := mustProject(t, "Website")

	t.Run("completing stamps the date", func(t *testing.T) {
		if err := p.SetStatus(StatusCompleted); err != nil {
			t.Fatal(err)
		}
		if p.CompletedAt == nil {
			t.Error("completing should stamp the date")
		}
	})

	t.Run("reopening clears the date", func(t *testing.T) {
		if err := p.SetStatus(StatusActive); err != nil {
			t.Fatal(err)
		}
		if p.CompletedAt != nil {
			t.Error("reopening should clear the date")
		}
	})

	t.Run("the infallible setters", func(t *testing.T) {
		// These exist so callers passing a known-good constant are not
		// left with an error branch that can never be taken.
		local := mustProject(t, "Website")
		local.Complete()
		if local.Status != StatusCompleted || local.CompletedAt == nil {
			t.Errorf("Complete: %+v", local)
		}
		local.Hold()
		if local.Status != StatusOnHold || local.CompletedAt != nil {
			t.Errorf("Hold: %+v", local)
		}
		local.Reopen()
		if local.Status != StatusActive || local.CompletedAt != nil {
			t.Errorf("Reopen: %+v", local)
		}
	})

	t.Run("an unknown status is rejected, not stored", func(t *testing.T) {
		if err := p.SetStatus("banana"); err == nil {
			t.Error("expected an error")
		}
		if p.Status != StatusActive {
			t.Errorf("Status = %q, want it unchanged", p.Status)
		}
	})
}

func TestProjectDeadlines(t *testing.T) {
	build := func(created, due Date) *Project {
		p, err := NewProject("P", "", due, created)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	sept1 := NewDate(2026, time.September, 1)

	t.Run("DaysUntilDue", func(t *testing.T) {
		p := build(sept1, NewDate(2026, time.September, 10))
		if days, ok := p.DaysUntilDue(fixedToday); !ok || days != 8 {
			t.Errorf("got (%d, %v), want (8, true)", days, ok)
		}

		overdue := build(sept1, NewDate(2026, time.August, 30))
		if days, _ := overdue.DaysUntilDue(fixedToday); days != -3 {
			t.Errorf("overdue days = %d, want -3", days)
		}

		// Zero days left is meaningful, so "no deadline" must be a
		// separate signal rather than a zero return.
		none := build(sept1, Date{})
		if days, ok := none.DaysUntilDue(fixedToday); ok || days != 0 {
			t.Errorf("got (%d, %v), want (0, false)", days, ok)
		}
	})

	t.Run("IsOverdue", func(t *testing.T) {
		if !build(sept1, NewDate(2026, time.August, 30)).IsOverdue(fixedToday) {
			t.Error("a passed deadline should be overdue")
		}
		if build(sept1, NewDate(2026, time.September, 30)).IsOverdue(fixedToday) {
			t.Error("a future deadline should not be overdue")
		}
		if build(sept1, Date{}).IsOverdue(fixedToday) {
			t.Error("no deadline can never be overdue")
		}

		completed := build(sept1, NewDate(2026, time.August, 1))
		if err := completed.SetStatus(StatusCompleted); err != nil {
			t.Fatal(err)
		}
		if completed.IsOverdue(fixedToday) {
			t.Error("a completed project is never overdue")
		}
	})

	t.Run("ScheduleElapsed", func(t *testing.T) {
		// Sep 1 -> Oct 1 is 30 days; today is Sep 2, so one day in.
		p := build(sept1, NewDate(2026, time.October, 1))
		got, ok := p.ScheduleElapsed(fixedToday)
		if !ok || got < 0.032 || got > 0.034 {
			t.Errorf("got (%v, %v), want about 1/30", got, ok)
		}

		if _, ok := build(sept1, Date{}).ScheduleElapsed(fixedToday); ok {
			t.Error("no deadline means no elapsed fraction")
		}

		sameDay := build(fixedToday, fixedToday)
		if got, _ := sameDay.ScheduleElapsed(fixedToday); got != 1 {
			t.Errorf("a same-day window = %v, want 1", got)
		}

		bounded := build(sept1, NewDate(2026, time.September, 10))
		if got, _ := bounded.ScheduleElapsed(NewDate(2026, time.August, 1)); got != 0 {
			t.Errorf("before the window = %v, want 0", got)
		}
		if got, _ := bounded.ScheduleElapsed(NewDate(2027, time.January, 1)); got != 1 {
			t.Errorf("after the window = %v, want 1", got)
		}
	})
}

func TestProjectRollups(t *testing.T) {
	setup := func(t *testing.T) (Projects, Tasks, *Project) {
		t.Helper()
		var projects Projects
		project, err := projects.Add("Website", "", Date{}, fixedToday)
		if err != nil {
			t.Fatal(err)
		}
		var tasks Tasks
		if _, err := tasks.Add("Design", 3, project.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := tasks.Add("Build", 5, project.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := tasks.Add("Buy milk", 1, ""); err != nil {
			t.Fatal(err)
		}
		return projects, tasks, project
	}

	t.Run("tasks are filed and unfiled correctly", func(t *testing.T) {
		_, tasks, project := setup(t)
		if got := len(tasks.ForProject(project.ID)); got != 2 {
			t.Errorf("ForProject = %d, want 2", got)
		}
		if got := len(tasks.Unfiled()); got != 1 {
			t.Errorf("Unfiled = %d, want 1", got)
		}
	})

	t.Run("TaskProgress", func(t *testing.T) {
		projects, tasks, project := setup(t)
		done, total, fraction := projects.TaskProgress(tasks, project.ID)
		if done != 0 || total != 2 || fraction != 0 {
			t.Errorf("got (%d, %d, %v), want (0, 2, 0)", done, total, fraction)
		}
		tasks[0].SetDone(true)
		done, total, fraction = projects.TaskProgress(tasks, project.ID)
		if done != 1 || total != 2 || fraction != 0.5 {
			t.Errorf("got (%d, %d, %v), want (1, 2, 0.5)", done, total, fraction)
		}
	})

	t.Run("a project with no tasks is zero, not a divide by zero", func(t *testing.T) {
		projects, tasks, _ := setup(t)
		empty, _ := projects.Add("Empty", "", Date{}, fixedToday)
		done, total, fraction := projects.TaskProgress(tasks, empty.ID)
		if done != 0 || total != 0 || fraction != 0 {
			t.Errorf("got (%d, %d, %v), want zeroes", done, total, fraction)
		}
		completed, estimated, effort := projects.EffortProgress(tasks, empty.ID)
		if completed != 0 || estimated != 0 || effort != 0 {
			t.Errorf("got (%d, %d, %v), want zeroes", completed, estimated, effort)
		}
	})

	t.Run("adding scope lowers the percentage", func(t *testing.T) {
		// Todoist behaviour: growing scope moves the bar backwards.
		// Nothing was undone; the project simply got bigger.
		projects, tasks, project := setup(t)
		tasks[0].SetDone(true)
		_, _, before := projects.TaskProgress(tasks, project.ID)
		if _, err := tasks.Add("Extra work", 1, project.ID); err != nil {
			t.Fatal(err)
		}
		_, _, after := projects.TaskProgress(tasks, project.ID)
		if after >= before {
			t.Errorf("after = %v, before = %v; scope growth should lower it", after, before)
		}
	})

	t.Run("EffortProgress sums estimates and completions", func(t *testing.T) {
		projects, tasks, project := setup(t)
		tasks[0].Completed = 1
		tasks[1].Completed = 1
		completed, estimated, fraction := projects.EffortProgress(tasks, project.ID)
		if completed != 2 || estimated != 8 || fraction != 0.25 {
			t.Errorf("got (%d, %d, %v), want (2, 8, 0.25)", completed, estimated, fraction)
		}
	})

	t.Run("effort is not capped at one", func(t *testing.T) {
		// Overrunning the estimate is the signal, not something to hide.
		projects, tasks, project := setup(t)
		tasks[0].Completed = 12
		_, _, fraction := projects.EffortProgress(tasks, project.ID)
		if fraction <= 1 {
			t.Errorf("fraction = %v, want more than 1", fraction)
		}
	})

	t.Run("the two measures can disagree", func(t *testing.T) {
		// Half the tasks done, but nearly all the effort spent: the
		// remaining half will hurt.
		projects, tasks, project := setup(t)
		tasks[0].SetDone(true)
		tasks[0].Completed = 7
		_, _, taskFraction := projects.TaskProgress(tasks, project.ID)
		_, _, effortFraction := projects.EffortProgress(tasks, project.ID)
		if taskFraction != 0.5 {
			t.Errorf("task fraction = %v, want 0.5", taskFraction)
		}
		if effortFraction <= 0.8 {
			t.Errorf("effort fraction = %v, want more than 0.8", effortFraction)
		}
	})

	t.Run("deleting a project keeps its tasks", func(t *testing.T) {
		projects, tasks, project := setup(t)
		moved := tasks.DetachFromProject(project.ID)
		if !projects.Delete(project.ID) {
			t.Error("Delete should report success")
		}
		if moved != 2 {
			t.Errorf("moved %d tasks, want 2", moved)
		}
		if len(tasks) != 3 || len(tasks.Unfiled()) != 3 {
			t.Errorf("tasks were lost: %d total, %d unfiled", len(tasks), len(tasks.Unfiled()))
		}
		if projects.Delete("nope") {
			t.Error("deleting an unknown id should report false")
		}
	})

	t.Run("detaching an unknown project moves nothing", func(t *testing.T) {
		_, tasks, _ := setup(t)
		if got := tasks.DetachFromProject("nope"); got != 0 {
			t.Errorf("moved %d, want 0", got)
		}
	})
}

func TestProjectHealth(t *testing.T) {
	build := func(t *testing.T, created, due Date) (Projects, *Project) {
		t.Helper()
		var projects Projects
		p, err := projects.Add("P", "", due, created)
		if err != nil {
			t.Fatal(err)
		}
		return projects, p
	}
	sept1 := NewDate(2026, time.September, 1)

	t.Run("completed and on hold win over everything", func(t *testing.T) {
		for _, tc := range []struct{ status, want string }{
			{StatusCompleted, HealthCompleted},
			{StatusOnHold, HealthOnHold},
		} {
			// Long overdue, yet the explicit status still wins.
			projects, p := build(t, sept1, NewDate(2026, time.January, 1))
			if err := p.SetStatus(tc.status); err != nil {
				t.Fatal(err)
			}
			if got := projects.Health(p, nil, fixedToday); got != tc.want {
				t.Errorf("status %q health = %q, want %q", tc.status, got, tc.want)
			}
		}
	})

	t.Run("overdue", func(t *testing.T) {
		projects, p := build(t, sept1, NewDate(2026, time.August, 30))
		if got := projects.Health(p, nil, fixedToday); got != HealthOverdue {
			t.Errorf("health = %q, want %q", got, HealthOverdue)
		}
	})

	t.Run("at risk when progress lags the schedule", func(t *testing.T) {
		// 80% of the schedule gone, one task of four done.
		projects, p := build(t, NewDate(2026, time.August, 23), NewDate(2026, time.September, 4))
		var tasks Tasks
		for i := 0; i < 4; i++ {
			if _, err := tasks.Add("t", 1, p.ID); err != nil {
				t.Fatal(err)
			}
		}
		tasks[0].SetDone(true)
		if got := projects.Health(p, tasks, fixedToday); got != HealthAtRisk {
			t.Errorf("health = %q, want %q", got, HealthAtRisk)
		}
	})

	t.Run("on track when progress keeps pace", func(t *testing.T) {
		projects, p := build(t, NewDate(2026, time.August, 23), NewDate(2026, time.September, 4))
		var tasks Tasks
		for i := 0; i < 4; i++ {
			if _, err := tasks.Add("t", 1, p.ID); err != nil {
				t.Fatal(err)
			}
		}
		for _, task := range tasks[:3] {
			task.SetDone(true)
		}
		if got := projects.Health(p, tasks, fixedToday); got != HealthOnTrack {
			t.Errorf("health = %q, want %q", got, HealthOnTrack)
		}
	})

	t.Run("not started with no activity", func(t *testing.T) {
		projects, p := build(t, sept1, Date{})
		var tasks Tasks
		if _, err := tasks.Add("t", 1, p.ID); err != nil {
			t.Fatal(err)
		}
		if got := projects.Health(p, tasks, fixedToday); got != HealthNotStarted {
			t.Errorf("health = %q, want %q", got, HealthNotStarted)
		}
	})

	t.Run("logged pomodoros count as started", func(t *testing.T) {
		// Work in progress is not "not started", even with nothing
		// finished.
		projects, p := build(t, sept1, Date{})
		var tasks Tasks
		if _, err := tasks.Add("t", 3, p.ID); err != nil {
			t.Fatal(err)
		}
		tasks[0].Completed = 1
		if got := projects.Health(p, tasks, fixedToday); got != HealthOnTrack {
			t.Errorf("health = %q, want %q", got, HealthOnTrack)
		}
	})

	t.Run("without a deadline nothing can be at risk", func(t *testing.T) {
		// With no deadline there is nothing to be behind schedule
		// against.
		projects, p := build(t, sept1, Date{})
		var tasks Tasks
		for i := 0; i < 4; i++ {
			if _, err := tasks.Add("t", 1, p.ID); err != nil {
				t.Fatal(err)
			}
		}
		tasks[0].SetDone(true)
		if got := projects.Health(p, tasks, fixedToday); got != HealthOnTrack {
			t.Errorf("health = %q, want %q", got, HealthOnTrack)
		}
	})
}

func TestProjectSummaryAndPortfolio(t *testing.T) {
	var projects Projects
	project, err := projects.Add("Website", "Blurb",
		NewDate(2026, time.September, 30), NewDate(2026, time.September, 1))
	if err != nil {
		t.Fatal(err)
	}
	var tasks Tasks
	design, _ := tasks.Add("Design", 3, project.ID)
	if _, err := tasks.Add("Build", 5, project.ID); err != nil {
		t.Fatal(err)
	}
	design.SetDone(true)
	design.Completed = 1
	sessions := Sessions{NewSession(time.Now(), time.Now(), 25, design, project)}

	t.Run("Summarize reports every field the UI needs", func(t *testing.T) {
		s := projects.Summarize(project, tasks, sessions, fixedToday)
		checks := []struct {
			name      string
			got, want any
		}{
			{"Name", s.Name, "Website"},
			{"Status", s.Status, StatusActive},
			{"Health", s.Health, HealthOnTrack},
			{"TasksDone", s.TasksDone, 1},
			{"TasksTotal", s.TasksTotal, 2},
			{"TaskFraction", s.TaskFraction, 0.5},
			{"PomodorosDone", s.PomodorosDone, 1},
			{"PomodorosEstimated", s.PomodorosEstimated, 8},
			{"Minutes", s.Minutes, 25},
			{"SessionCount", s.SessionCount, 1},
			{"DaysLeft", s.DaysLeft, 28},
			{"HasDeadline", s.HasDeadline, true},
			{"Overdue", s.Overdue, false},
		}
		for _, c := range checks {
			if c.got != c.want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		}
	})

	t.Run("Find and NameOf", func(t *testing.T) {
		if got, err := projects.Find(project.ID); err != nil || got != project {
			t.Errorf("Find = %v, %v", got, err)
		}
		if _, err := projects.Find("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
		if got := projects.NameOf(project.ID, "—"); got != "Website" {
			t.Errorf("NameOf = %q", got)
		}
		if got := projects.NameOf("", "—"); got != "—" {
			t.Errorf("NameOf(unknown) = %q, want the fallback", got)
		}
	})

	t.Run("Move and Active", func(t *testing.T) {
		local := Projects{}
		a, _ := local.Add("A", "", Date{}, fixedToday)
		b, _ := local.Add("B", "", Date{}, fixedToday)
		c, _ := local.Add("C", "", Date{}, fixedToday)

		if _, err := local.Move(c.ID, -1); err != nil {
			t.Fatal(err)
		}
		if local[1] != c {
			t.Error("Move did not reorder")
		}
		if idx, _ := local.Move(a.ID, -1); idx != 0 {
			t.Errorf("clamped index = %d, want 0", idx)
		}
		if _, err := local.Move("nope", 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}

		if err := b.SetStatus(StatusOnHold); err != nil {
			t.Fatal(err)
		}
		if got := len(local.Active()); got != 2 {
			t.Errorf("Active = %d, want 2", got)
		}
	})

	t.Run("Add rejects a blank name", func(t *testing.T) {
		local := Projects{}
		if _, err := local.Add("  ", "", Date{}, fixedToday); err == nil {
			t.Error("expected an error")
		}
		if len(local) != 0 {
			t.Error("a rejected project should not be appended")
		}
	})

	t.Run("PortfolioStats", func(t *testing.T) {
		local := Projects{}
		if _, err := local.Add("Active", "", Date{}, fixedToday); err != nil {
			t.Fatal(err)
		}
		held, _ := local.Add("Held", "", Date{}, fixedToday)
		done, _ := local.Add("Done", "", Date{}, fixedToday)
		late, _ := local.Add("Late", "", NewDate(2026, time.August, 1), NewDate(2026, time.July, 1))
		_ = late
		if err := held.SetStatus(StatusOnHold); err != nil {
			t.Fatal(err)
		}
		if err := done.SetStatus(StatusCompleted); err != nil {
			t.Fatal(err)
		}

		active, completed, needAttention := local.PortfolioStats(nil, fixedToday)
		if active != 2 || completed != 1 || needAttention != 1 {
			t.Errorf("got (%d, %d, %d), want (2, 1, 1)", active, completed, needAttention)
		}
	})
}

func TestProjectHierarchy(t *testing.T) {
	setup := func(t *testing.T) (Projects, *Project, *Project, *Project) {
		t.Helper()
		var projects Projects
		website, err := projects.Add("Website", "", Date{}, fixedToday)
		if err != nil {
			t.Fatal(err)
		}
		redesign, err := projects.Add("Redesign", "", Date{}, fixedToday)
		if err != nil {
			t.Fatal(err)
		}
		checkout, err := projects.Add("Checkout", "", Date{}, fixedToday)
		if err != nil {
			t.Fatal(err)
		}
		if err := projects.SetParent(redesign.ID, website.ID); err != nil {
			t.Fatal(err)
		}
		if err := projects.SetParent(checkout.ID, redesign.ID); err != nil {
			t.Fatal(err)
		}
		return projects, website, redesign, checkout
	}

	t.Run("a fresh project is top-level", func(t *testing.T) {
		p := mustProject(t, "Solo")
		if p.ParentID != "" {
			t.Errorf("ParentID = %q, want empty", p.ParentID)
		}
	})

	t.Run("SetParent files a project under another", func(t *testing.T) {
		_, website, redesign, _ := setup(t)
		if redesign.ParentID != website.ID {
			t.Errorf("ParentID = %q, want %q", redesign.ParentID, website.ID)
		}
	})

	t.Run("SetParent with an empty id clears it", func(t *testing.T) {
		projects, website, redesign, _ := setup(t)
		if err := projects.SetParent(redesign.ID, ""); err != nil {
			t.Fatal(err)
		}
		if redesign.ParentID != "" {
			t.Error("expected redesign to become top-level")
		}
		_ = website
	})

	t.Run("SetParent rejects an unknown parent", func(t *testing.T) {
		projects, website, _, _ := setup(t)
		if err := projects.SetParent(website.ID, "no-such-project"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("SetParent rejects an unknown project", func(t *testing.T) {
		projects, _, _, _ := setup(t)
		if err := projects.SetParent("no-such-project", ""); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("SetParent rejects a project as its own parent", func(t *testing.T) {
		projects, website, _, _ := setup(t)
		if err := projects.SetParent(website.ID, website.ID); err == nil {
			t.Error("expected an error")
		}
		if website.ParentID != "" {
			t.Error("a rejected assignment must not be applied")
		}
	})

	t.Run("SetParent rejects a cycle through a descendant", func(t *testing.T) {
		projects, website, _, checkout := setup(t)
		// website -> redesign -> checkout already; filing website under
		// its own grandchild would close the loop.
		if err := projects.SetParent(website.ID, checkout.ID); err == nil {
			t.Error("expected an error")
		}
		if website.ParentID != "" {
			t.Error("a rejected assignment must not be applied")
		}
	})

	t.Run("Children and Descendants", func(t *testing.T) {
		projects, website, redesign, checkout := setup(t)
		if got := projects.Children(website.ID); len(got) != 1 || got[0] != redesign {
			t.Errorf("Children(website) = %v, want [redesign]", got)
		}
		if got := projects.Children(""); len(got) != 1 || got[0] != website {
			t.Errorf("Children(\"\") = %v, want [website]", got)
		}
		descendants := projects.Descendants(website.ID)
		if len(descendants) != 2 || descendants[0] != redesign || descendants[1] != checkout {
			t.Errorf("Descendants(website) = %v, want [redesign checkout]", descendants)
		}
		if got := projects.Descendants(checkout.ID); len(got) != 0 {
			t.Errorf("a leaf project has descendants: %v", got)
		}
	})

	t.Run("Path renders the breadcrumb", func(t *testing.T) {
		projects, _, _, checkout := setup(t)
		if got := projects.Path(checkout); got != "Website > Redesign > Checkout" {
			t.Errorf("Path = %q", got)
		}
	})

	t.Run("Path stops at a dangling parent reference", func(t *testing.T) {
		// Only a hand-edited or corrupted file produces this; SetParent
		// can't. Path must still return something rather than recursing
		// into a project that is not there.
		orphan := &Project{ID: "x", Name: "Orphan", ParentID: "no-such-project"}
		projects := Projects{orphan}
		if got := projects.Path(orphan); got != "Orphan" {
			t.Errorf("Path = %q, want %q", got, "Orphan")
		}
	})

	t.Run("a dangling reference partway up the chain is not a cycle", func(t *testing.T) {
		// hasCycle must be able to tell "the chain runs off the edge of
		// the known projects" apart from "the chain loops": SetParent
		// only validates the immediate parent, so a corrupted grandparent
		// reference must not block filing something under a project
		// whose own ancestry is broken further up.
		corrupt := &Project{ID: "c", Name: "Corrupt", ParentID: "no-such-project"}
		projects := Projects{corrupt}
		leaf := mustProject(t, "Leaf")
		projects = append(projects, leaf)
		if err := projects.SetParent(leaf.ID, corrupt.ID); err != nil {
			t.Errorf("SetParent should not treat a dangling ancestor as a cycle: %v", err)
		}
	})

	t.Run("progress rolls subproject tasks up into the parent", func(t *testing.T) {
		projects, website, redesign, checkout := setup(t)
		var tasks Tasks
		if _, err := tasks.Add("Own task", 2, website.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := tasks.Add("Redesign task", 3, redesign.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := tasks.Add("Checkout task", 1, checkout.ID); err != nil {
			t.Fatal(err)
		}
		tasks[2].SetDone(true) // the checkout task, two levels down

		done, total, _ := projects.TaskProgress(tasks, website.ID)
		if done != 1 || total != 3 {
			t.Errorf("TaskProgress(website) = (%d, %d), want (1, 3)", done, total)
		}
		_, estimated, _ := projects.EffortProgress(tasks, website.ID)
		if estimated != 6 {
			t.Errorf("EffortProgress(website) estimated = %d, want 6", estimated)
		}

		// The subproject's own totals are unaffected by its parent's.
		done, total, _ = projects.TaskProgress(tasks, redesign.ID)
		if done != 1 || total != 2 {
			t.Errorf("TaskProgress(redesign) = (%d, %d), want (1, 2)", done, total)
		}
	})

	t.Run("Delete promotes direct children to the deleted project's parent", func(t *testing.T) {
		projects, website, redesign, checkout := setup(t)
		if !projects.Delete(redesign.ID) {
			t.Fatal("Delete should report success")
		}
		if checkout.ParentID != website.ID {
			t.Errorf("checkout.ParentID = %q, want %q (promoted)", checkout.ParentID, website.ID)
		}
	})

	t.Run("Delete promotes children to top-level when the deleted project had no parent", func(t *testing.T) {
		projects, website, redesign, _ := setup(t)
		if !projects.Delete(website.ID) {
			t.Fatal("Delete should report success")
		}
		if redesign.ParentID != "" {
			t.Errorf("redesign.ParentID = %q, want empty", redesign.ParentID)
		}
	})

	t.Run("repairHierarchy clears a dangling parent reference", func(t *testing.T) {
		projects := Projects{{ID: "a", ParentID: "no-such-project"}}
		projects.repairHierarchy()
		if projects[0].ParentID != "" {
			t.Errorf("ParentID = %q, want cleared", projects[0].ParentID)
		}
	})

	t.Run("repairHierarchy breaks a cycle written directly to the field", func(t *testing.T) {
		// SetParent cannot produce this; only a hand-edited or corrupted
		// file can. repairHierarchy is the safety net for that case.
		projects := Projects{
			{ID: "a", ParentID: "b"},
			{ID: "b", ParentID: "a"},
		}
		projects.repairHierarchy()
		if projects[0].ParentID != "" {
			t.Errorf("a.ParentID = %q, want cleared", projects[0].ParentID)
		}
		if projects[1].ParentID != "a" {
			t.Errorf("b.ParentID = %q, want unchanged (a is now top-level)", projects[1].ParentID)
		}
	})

	t.Run("repairHierarchy clears a project set as its own parent", func(t *testing.T) {
		projects := Projects{{ID: "a", ParentID: "a"}}
		projects.repairHierarchy()
		if projects[0].ParentID != "" {
			t.Errorf("ParentID = %q, want cleared", projects[0].ParentID)
		}
	})
}
