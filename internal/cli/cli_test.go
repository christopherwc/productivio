package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/christopherwc/pomodoro/internal/core"
)

// fixedNow is a Wednesday, pinned so streak, deadline and health output
// cannot vary with the day the suite runs.
var fixedNow = time.Date(2026, time.September, 2, 9, 0, 0, 0, time.Local)

// harness is an Env over a temporary directory with a pinned clock and
// no real sleeping or notifying, plus captured output.
type harness struct {
	env    *Env
	out    *bytes.Buffer
	errOut *bytes.Buffer

	// slept records the total simulated wait, so countdown behaviour
	// can be asserted without the suite actually waiting.
	slept    time.Duration
	notified []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store, err := core.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	h := &harness{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	h.env = &Env{
		Store: store,
		Out:   h.out,
		Err:   h.errOut,
		Now:   func() time.Time { return fixedNow },
		Sleep: func(d time.Duration) { h.slept += d },
		Notify: func(title, _ string) bool {
			h.notified = append(h.notified, title)
			return true
		},
	}
	return h
}

// run dispatches a command and returns the exit code, resetting the
// output buffers first so each assertion sees only its own output.
func (h *harness) run(args ...string) int {
	h.out.Reset()
	h.errOut.Reset()
	return Dispatch(h.env, args)
}

func (h *harness) stdout() string { return h.out.String() }
func (h *harness) stderr() string { return h.errOut.String() }

// firstID pulls an id out of an "Added <id>  <name>" confirmation line.
func firstID(t *testing.T, output string) string {
	t.Helper()
	fields := strings.Fields(output)
	if len(fields) < 2 || fields[0] != "Added" {
		t.Fatalf("cannot find an id in %q", output)
	}
	return fields[1]
}

func TestDispatchBasics(t *testing.T) {
	t.Run("no arguments prints usage", func(t *testing.T) {
		h := newHarness(t)
		if code := h.run(); code != exitUsage {
			t.Errorf("exit code = %d, want %d", code, exitUsage)
		}
		if !strings.Contains(h.stdout(), "Usage:") {
			t.Errorf("output = %q, want usage", h.stdout())
		}
	})

	t.Run("help", func(t *testing.T) {
		for _, arg := range []string{"help", "-h", "--help"} {
			h := newHarness(t)
			if code := h.run(arg); code != exitOK {
				t.Errorf("%s exit code = %d, want 0", arg, code)
			}
			if !strings.Contains(h.stdout(), "Usage:") {
				t.Errorf("%s did not print usage", arg)
			}
		}
	})

	t.Run("an unknown command is a usage error", func(t *testing.T) {
		h := newHarness(t)
		if code := h.run("frobnicate"); code != exitUsage {
			t.Errorf("exit code = %d, want %d", code, exitUsage)
		}
		if !strings.Contains(h.stderr(), "unknown command") {
			t.Errorf("stderr = %q", h.stderr())
		}
	})

	t.Run("where and version", func(t *testing.T) {
		h := newHarness(t)
		h.run("where")
		if strings.TrimSpace(h.stdout()) != h.env.Store.Dir() {
			t.Errorf("where printed %q, want the data directory", h.stdout())
		}
		h.run("version")
		if !strings.Contains(h.stdout(), "pomodoro") || !strings.Contains(h.stdout(), "Go ") {
			t.Errorf("version printed %q", h.stdout())
		}
	})
}

func TestStatus(t *testing.T) {
	h := newHarness(t)

	t.Run("empty state", func(t *testing.T) {
		if code := h.run("status"); code != exitOK {
			t.Fatalf("exit code = %d", code)
		}
		for _, want := range []string{
			"Today: 0 pomodoros",
			"0 open, 0 done",
			"none scheduled today",
			"0 active, 0 completed",
		} {
			if !strings.Contains(h.stdout(), want) {
				t.Errorf("status missing %q:\n%s", want, h.stdout())
			}
		}
	})

	t.Run("reports due habits and projects needing attention", func(t *testing.T) {
		h.run("habit", "add", "Meditate", "daily")
		h.run("project", "add", "Late", "2026-08-01")
		h.run("status")
		if !strings.Contains(h.stdout(), "due: Meditate") {
			t.Errorf("status should name the due habit:\n%s", h.stdout())
		}
		if !strings.Contains(h.stdout(), "needing attention") {
			t.Errorf("status should flag the overdue project:\n%s", h.stdout())
		}
	})
}

func TestTaskCommands(t *testing.T) {
	h := newHarness(t)

	t.Run("list is empty at first", func(t *testing.T) {
		h.run("task", "list")
		if !strings.Contains(h.stdout(), "No tasks yet") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	h.run("project", "add", "Website")
	projectID := firstID(t, h.stdout())

	t.Run("add with an estimate and a project", func(t *testing.T) {
		if code := h.run("task", "add", "Design", "3", projectID); code != exitOK {
			t.Fatalf("exit code = %d: %s", code, h.stderr())
		}
		tasks := h.env.Store.LoadTasks()
		if len(tasks) != 1 || tasks[0].Estimate != 3 || tasks[0].ProjectID != projectID {
			t.Errorf("task was not filed correctly: %+v", tasks)
		}
	})

	t.Run("add defaults to one pomodoro and no project", func(t *testing.T) {
		h.run("task", "add", "Buy milk")
		tasks := h.env.Store.LoadTasks()
		last := tasks[len(tasks)-1]
		if last.Estimate != 1 || last.ProjectID != "" {
			t.Errorf("defaults wrong: %+v", last)
		}
	})

	t.Run("list marks completed tasks", func(t *testing.T) {
		taskID := h.env.Store.LoadTasks()[0].ID
		h.run("task", "done", taskID)
		h.run("task", "list")
		if !strings.Contains(h.stdout(), "x") {
			t.Errorf("a completed task should be marked:\n%s", h.stdout())
		}
		h.run("task", "done", taskID) // put it back
	})

	t.Run("list shows the project name", func(t *testing.T) {
		h.run("task", "list")
		if !strings.Contains(h.stdout(), "Website") {
			t.Errorf("list should show the project:\n%s", h.stdout())
		}
		if !strings.Contains(h.stdout(), "Buy milk") {
			t.Errorf("list should show the unfiled task:\n%s", h.stdout())
		}
	})

	t.Run("done toggles both ways", func(t *testing.T) {
		taskID := h.env.Store.LoadTasks()[0].ID
		h.run("task", "done", taskID)
		if !h.env.Store.LoadTasks()[0].Done {
			t.Error("task should be done")
		}
		if !strings.Contains(h.stdout(), "completed") {
			t.Errorf("output = %q", h.stdout())
		}
		h.run("task", "done", taskID)
		if h.env.Store.LoadTasks()[0].Done {
			t.Error("task should be reopened")
		}
		if !strings.Contains(h.stdout(), "reopened") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	t.Run("rm deletes", func(t *testing.T) {
		taskID := h.env.Store.LoadTasks()[0].ID
		before := len(h.env.Store.LoadTasks())
		h.run("task", "rm", taskID)
		if got := len(h.env.Store.LoadTasks()); got != before-1 {
			t.Errorf("tasks = %d, want %d", got, before-1)
		}
	})

	t.Run("argument errors", func(t *testing.T) {
		cases := [][]string{
			{"task"},
			{"task", "wat"},
			{"task", "add"},
			{"task", "add", "  "},
			{"task", "add", "T", "not-a-number"},
			{"task", "add", "T", "1", "no-such-project"},
			{"task", "done"},
			{"task", "done", "no-such-task"},
			{"task", "rm"},
			{"task", "rm", "no-such-task"},
		}
		for _, args := range cases {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				if code := h.run(args...); code == exitOK {
					t.Errorf("expected a non-zero exit code")
				}
				if h.stderr() == "" {
					t.Error("expected something on stderr")
				}
			})
		}
	})
}

func TestProjectCommands(t *testing.T) {
	h := newHarness(t)

	t.Run("list is empty at first", func(t *testing.T) {
		h.run("project", "list")
		if !strings.Contains(h.stdout(), "No projects yet") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	h.run("project", "add", "Website", "2026-09-30")
	projectID := firstID(t, h.stdout())

	t.Run("list shows both progress measures", func(t *testing.T) {
		h.run("task", "add", "Design", "4", projectID)
		taskID := h.env.Store.LoadTasks()[0].ID
		h.run("task", "done", taskID)

		h.run("project", "list")
		out := h.stdout()
		if !strings.Contains(out, "1/1 (100%)") {
			t.Errorf("task progress missing:\n%s", out)
		}
		if !strings.Contains(out, "0/4") {
			t.Errorf("effort progress missing:\n%s", out)
		}
		if !strings.Contains(out, "2026-09-30") {
			t.Errorf("deadline missing:\n%s", out)
		}
	})

	t.Run("a project without a deadline shows a dash", func(t *testing.T) {
		h.run("project", "add", "No deadline")
		h.run("project", "list")
		if !strings.Contains(h.stdout(), "No deadline") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	t.Run("done marks it complete", func(t *testing.T) {
		h.run("project", "done", projectID)
		project, err := h.env.Store.LoadProjects().Find(projectID)
		if err != nil {
			t.Fatal(err)
		}
		if project.Status != core.StatusCompleted {
			t.Errorf("status = %q, want completed", project.Status)
		}
	})

	t.Run("argument errors", func(t *testing.T) {
		cases := [][]string{
			{"project"},
			{"project", "wat"},
			{"project", "add"},
			{"project", "add", "   "},
			{"project", "add", "P", "not-a-date"},
			{"project", "done"},
			{"project", "done", "no-such-project"},
		}
		for _, args := range cases {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				if code := h.run(args...); code == exitOK {
					t.Error("expected a non-zero exit code")
				}
			})
		}
	})
}

func TestHabitCommands(t *testing.T) {
	h := newHarness(t)

	t.Run("list is empty at first", func(t *testing.T) {
		h.run("habit", "list")
		if !strings.Contains(h.stdout(), "No habits yet") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	t.Run("add accepts every schedule preset", func(t *testing.T) {
		for _, schedule := range []string{"daily", "weekdays", "weekends", "Every-Day"} {
			if code := h.run("habit", "add", "Habit "+schedule, schedule); code != exitOK {
				t.Errorf("%s failed: %s", schedule, h.stderr())
			}
		}
		h.run("habit", "add", "Default schedule")
		habits := h.env.Store.LoadHabits()
		if len(habits) != 5 {
			t.Fatalf("added %d habits, want 5", len(habits))
		}
		if habits[4].ScheduleLabel() != "Every day" {
			t.Errorf("default schedule = %q", habits[4].ScheduleLabel())
		}
	})

	t.Run("check toggles today and reports the streak", func(t *testing.T) {
		habitID := h.env.Store.LoadHabits()[0].ID
		h.run("habit", "check", habitID)
		if !strings.Contains(h.stdout(), "kept today (streak 1)") {
			t.Errorf("output = %q", h.stdout())
		}
		h.run("habit", "check", habitID)
		if !strings.Contains(h.stdout(), "cleared today") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	t.Run("list renders the recent grid", func(t *testing.T) {
		habitID := h.env.Store.LoadHabits()[0].ID
		h.run("habit", "check", habitID)
		h.run("habit", "list")
		out := h.stdout()
		if !strings.Contains(out, "STREAK") || !strings.Contains(out, "#") {
			t.Errorf("grid missing:\n%s", out)
		}
	})

	t.Run("argument errors", func(t *testing.T) {
		cases := [][]string{
			{"habit"},
			{"habit", "wat"},
			{"habit", "add"},
			{"habit", "add", "  "},
			{"habit", "add", "H", "fortnightly"},
			{"habit", "check"},
			{"habit", "check", "no-such-habit"},
		}
		for _, args := range cases {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				if code := h.run(args...); code == exitOK {
					t.Error("expected a non-zero exit code")
				}
			})
		}
	})
}

func TestStartCommand(t *testing.T) {
	t.Run("records an unattributed session", func(t *testing.T) {
		h := newHarness(t)
		if code := h.run("start", "-work", "1", "-rest", "1"); code != exitOK {
			t.Fatalf("exit code = %d: %s", code, h.stderr())
		}
		sessions := h.env.Store.LoadSessions()
		if len(sessions) != 1 {
			t.Fatalf("recorded %d sessions, want 1", len(sessions))
		}
		if sessions[0].WorkMinutes != 1 || sessions[0].TaskID != "" {
			t.Errorf("session = %+v", sessions[0])
		}
		// One tick per second of work plus rest, all simulated.
		if want := 120 * time.Second; h.slept != want {
			t.Errorf("slept %v, want %v", h.slept, want)
		}
		if len(h.notified) != 2 {
			t.Errorf("notifications = %v, want two", h.notified)
		}
	})

	t.Run("credits the task and its project", func(t *testing.T) {
		// The integration that makes focus time meaningful: naming a
		// task also attributes the interval to its project.
		h := newHarness(t)
		h.run("project", "add", "Website")
		projectID := firstID(t, h.stdout())
		h.run("task", "add", "Design", "3", projectID)
		taskID := h.env.Store.LoadTasks()[0].ID

		if code := h.run("start", "-work", "1", "-rest", "1", "-task", taskID); code != exitOK {
			t.Fatalf("exit code = %d: %s", code, h.stderr())
		}
		if !strings.Contains(h.stdout(), "Design · Website") {
			t.Errorf("should name task and project:\n%s", h.stdout())
		}

		session := h.env.Store.LoadSessions()[0]
		if session.TaskID != taskID || session.ProjectID != projectID {
			t.Errorf("attribution wrong: %+v", session)
		}
		if session.TaskTitle != "Design" || session.ProjectName != "Website" {
			t.Errorf("names not denormalised: %+v", session)
		}
		if got := h.env.Store.LoadTasks()[0].Completed; got != 1 {
			t.Errorf("task credited %d pomodoros, want 1", got)
		}
	})

	t.Run("an unfiled task credits no project", func(t *testing.T) {
		h := newHarness(t)
		h.run("task", "add", "Loose")
		taskID := h.env.Store.LoadTasks()[0].ID
		h.run("start", "-work", "1", "-rest", "1", "-task", taskID)
		if session := h.env.Store.LoadSessions()[0]; session.ProjectID != "" {
			t.Errorf("project = %q, want none", session.ProjectID)
		}
	})

	t.Run("uses the default intervals", func(t *testing.T) {
		h := newHarness(t)
		h.run("start")
		if got := h.env.Store.LoadSessions()[0].WorkMinutes; got != 25 {
			t.Errorf("WorkMinutes = %d, want the default 25", got)
		}
	})

	t.Run("long flag spellings are accepted", func(t *testing.T) {
		h := newHarness(t)
		h.run("start", "--work", "2", "--rest", "1")
		if got := h.env.Store.LoadSessions()[0].WorkMinutes; got != 2 {
			t.Errorf("WorkMinutes = %d, want 2", got)
		}
	})

	t.Run("argument errors", func(t *testing.T) {
		cases := [][]string{
			{"start", "-work"},
			{"start", "-work", "abc"},
			{"start", "-rest"},
			{"start", "-rest", "abc"},
			{"start", "-task"},
			{"start", "-task", "no-such-task"},
			{"start", "-work", "0"},
			{"start", "-rest", "0"},
			{"start", "-nonsense"},
		}
		for _, args := range cases {
			t.Run(strings.Join(args, " "), func(t *testing.T) {
				h := newHarness(t)
				if code := h.run(args...); code == exitOK {
					t.Error("expected a non-zero exit code")
				}
				if len(h.env.Store.LoadSessions()) != 0 {
					t.Error("a failed start must not record a session")
				}
			})
		}
	})
}

func TestHistoryCommand(t *testing.T) {
	h := newHarness(t)

	t.Run("empty", func(t *testing.T) {
		h.run("history")
		if !strings.Contains(h.stdout(), "No sessions recorded yet") {
			t.Errorf("output = %q", h.stdout())
		}
	})

	h.run("project", "add", "Website")
	projectID := firstID(t, h.stdout())
	h.run("task", "add", "Design", "3", projectID)
	taskID := h.env.Store.LoadTasks()[0].ID
	h.run("start", "-work", "1", "-rest", "1", "-task", taskID)
	h.run("start", "-work", "2", "-rest", "1")

	t.Run("lists newest first with attribution", func(t *testing.T) {
		h.run("history")
		out := h.stdout()
		if !strings.Contains(out, "Design") || !strings.Contains(out, "Website") {
			t.Errorf("attribution missing:\n%s", out)
		}
		// The unattributed session is newest, so its dash comes first.
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected a header and two rows:\n%s", out)
		}
		if !strings.Contains(lines[1], "-") {
			t.Errorf("newest row should be the unattributed one: %q", lines[1])
		}
	})

	t.Run("honours a limit", func(t *testing.T) {
		h.run("history", "1")
		if lines := strings.Split(strings.TrimSpace(h.stdout()), "\n"); len(lines) != 2 {
			t.Errorf("expected a header and one row:\n%s", h.stdout())
		}
	})

	t.Run("a limit beyond the history shows everything", func(t *testing.T) {
		h.run("history", "500")
		if lines := strings.Split(strings.TrimSpace(h.stdout()), "\n"); len(lines) != 3 {
			t.Errorf("expected a header and two rows:\n%s", h.stdout())
		}
	})

	t.Run("argument errors", func(t *testing.T) {
		for _, arg := range []string{"abc", "0", "-3"} {
			if code := h.run("history", arg); code == exitOK {
				t.Errorf("history %q should have failed", arg)
			}
		}
	})
}

// failingStore wraps a real store but refuses every write.
//
// Reads still succeed, which is the combination that matters: the
// command must load its records, attempt to persist a change, and
// report the failure rather than claiming success.
type failingStore struct {
	Store
	err error
}

func (f failingStore) SaveSessions(core.Sessions) error { return f.err }
func (f failingStore) SaveTasks(core.Tasks) error       { return f.err }
func (f failingStore) SaveHabits(core.Habits) error     { return f.err }
func (f failingStore) SaveProjects(core.Projects) error { return f.err }

// taskFailingStore refuses only task writes. During `start -task` the
// session is saved first, so a store that failed everything would stop
// before the task credit was ever attempted.
type taskFailingStore struct {
	Store
	err error
}

func (f taskFailingStore) SaveTasks(core.Tasks) error { return f.err }

func TestStartReportsAFailedTaskCredit(t *testing.T) {
	h := newHarness(t)
	h.run("task", "add", "Design", "3")
	taskID := h.env.Store.LoadTasks()[0].ID
	h.env.Store = taskFailingStore{Store: h.env.Store, err: errors.New("disk full")}

	if code := h.run("start", "-work", "1", "-rest", "1", "-task", taskID); code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(h.stderr(), "disk full") {
		t.Errorf("stderr = %q, want the underlying error", h.stderr())
	}
	// The session itself was saved before the credit failed, so the
	// work is not lost even though the count could not be updated.
	if len(h.env.Store.LoadSessions()) != 1 {
		t.Error("the session should still have been recorded")
	}
}

// TestSaveFailuresSurface checks that a write error reaches the user
// rather than being swallowed, for every command that persists.
//
// A save that silently does nothing is the worst outcome available: the
// user believes their work is recorded when it is not.
func TestSaveFailuresSurface(t *testing.T) {
	setup := func(h *harness) map[string]string {
		// Create one of everything first, while writes still work.
		h.run("project", "add", "Website")
		projectID := strings.Fields(h.stdout())[1]
		h.run("task", "add", "Design", "3", projectID)
		h.run("habit", "add", "Meditate", "daily")
		return map[string]string{
			"project": projectID,
			"task":    h.env.Store.LoadTasks()[0].ID,
			"habit":   h.env.Store.LoadHabits()[0].ID,
		}
	}

	commands := map[string]func(ids map[string]string) []string{
		"task add":     func(map[string]string) []string { return []string{"task", "add", "New"} },
		"task done":    func(i map[string]string) []string { return []string{"task", "done", i["task"]} },
		"task rm":      func(i map[string]string) []string { return []string{"task", "rm", i["task"]} },
		"project add":  func(map[string]string) []string { return []string{"project", "add", "New"} },
		"project done": func(i map[string]string) []string { return []string{"project", "done", i["project"]} },
		"habit add":    func(map[string]string) []string { return []string{"habit", "add", "New"} },
		"habit check":  func(i map[string]string) []string { return []string{"habit", "check", i["habit"]} },
		"start":        func(map[string]string) []string { return []string{"start", "-work", "1", "-rest", "1"} },
		"start -task": func(i map[string]string) []string {
			return []string{"start", "-work", "1", "-rest", "1", "-task", i["task"]}
		},
	}

	for name, build := range commands {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			ids := setup(h)
			h.env.Store = failingStore{Store: h.env.Store, err: errors.New("disk full")}

			if code := h.run(build(ids)...); code != exitError {
				t.Errorf("exit code = %d, want %d", code, exitError)
			}
			if !strings.Contains(h.stderr(), "disk full") {
				t.Errorf("stderr = %q, want the underlying error", h.stderr())
			}
		})
	}
}

func TestRunUsesTheRealEnvironment(t *testing.T) {
	// Run is the only function that touches the real data directory, so
	// it is exercised with HOME pointed at a temporary one.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	var out, errOut bytes.Buffer
	if code := Run([]string{"where"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit code = %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), home) {
		t.Errorf("where printed %q, want a path under the temporary home", out.String())
	}
}

func TestRunMigratesLegacyHistory(t *testing.T) {
	// Upgrading from the Python version must not lose history.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	legacy := filepath.Join(home, core.LegacySessionsFile)
	if err := os.WriteFile(legacy, []byte(
		`[{"start":"2026-08-01T09:00:00","end":"2026-08-01T09:25:00","work_minutes":25}]`),
		0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := Run([]string{"history"}, &out, &errOut); code != exitOK {
		t.Fatalf("exit code = %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Imported your existing session history") {
		t.Errorf("expected a migration notice:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "2026-08-01") {
		t.Errorf("migrated session missing:\n%s", out.String())
	}
}

func TestRunReportsAnUnknownHomeDirectory(t *testing.T) {
	// With no home directory there is nowhere to keep data; the program
	// must explain that rather than panicking.
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	var out, errOut bytes.Buffer
	if code := Run([]string{"status"}, &out, &errOut); code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if errOut.String() == "" {
		t.Error("expected an explanation on stderr")
	}
}

func TestRunReportsAnUnusableDataDirectory(t *testing.T) {
	// A home directory that is a regular file cannot hold a data
	// directory; the program must say so rather than panic.
	blocked := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", blocked)
	t.Setenv("XDG_DATA_HOME", filepath.Join(blocked, "data"))

	var out, errOut bytes.Buffer
	if code := Run([]string{"status"}, &out, &errOut); code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if errOut.String() == "" {
		t.Error("expected an explanation on stderr")
	}
}
