package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStore returns a Store over a fresh temporary directory, so no
// test can ever touch real user data. t.TempDir is removed
// automatically when the test finishes.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestStoreRoundTrip(t *testing.T) {
	store := newTestStore(t)

	start := time.Date(2026, time.September, 2, 9, 0, 0, 0, time.Local)
	end := start.Add(25 * time.Minute)
	sessions := Sessions{NewSession(start, end, 25, nil, nil)}

	if err := store.SaveSessions(sessions); err != nil {
		t.Fatalf("SaveSessions: %v", err)
	}
	loaded := store.LoadSessions()
	if len(loaded) != 1 {
		t.Fatalf("loaded %d sessions, want 1", len(loaded))
	}
	if loaded[0].WorkMinutes != 25 {
		t.Errorf("WorkMinutes = %d, want 25", loaded[0].WorkMinutes)
	}
	if got := loaded[0].Start.String(); got != "2026-09-02T09:00:00" {
		t.Errorf("Start = %q, want the naive Python layout", got)
	}
}

func TestStoreTolerance(t *testing.T) {
	// A missing, corrupt or wrongly-shaped file must not stop the
	// application from starting. A corrupted history is a bad day; a
	// program that refuses to open is a worse one.
	tests := []struct {
		name    string
		content string
	}{
		{"corrupt json", "{not valid json!!"},
		{"wrong shape", `{"oops": true}`},
		{"binary rubbish", "\xff\xfe\x00\x01"},
		{"empty file", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			write(t, store.Path(SessionsFile), tc.content)
			if got := store.LoadSessions(); len(got) != 0 {
				t.Errorf("loaded %d sessions from a bad file, want 0", len(got))
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		store := newTestStore(t)
		if got := store.LoadSessions(); len(got) != 0 {
			t.Errorf("loaded %d sessions with no file, want 0", len(got))
		}
	})
}

func TestStoreWritesAreAtomic(t *testing.T) {
	store := newTestStore(t)

	t.Run("a successful write leaves no temp files", func(t *testing.T) {
		if err := store.SaveTasks(Tasks{mustTask(t, "Design", 3, "")}); err != nil {
			t.Fatalf("SaveTasks: %v", err)
		}
		if leftovers := tempFiles(t, store.Dir()); len(leftovers) != 0 {
			t.Errorf("atomic write left %v behind", leftovers)
		}
	})

	t.Run("a failed write preserves the previous file", func(t *testing.T) {
		// This is the entire point of writing through a temp file.
		good := Tasks{mustTask(t, "Keep me", 1, "")}
		if err := store.SaveTasks(good); err != nil {
			t.Fatalf("SaveTasks: %v", err)
		}

		// A channel cannot be marshalled, so encoding fails before
		// anything is written.
		if err := store.writeJSON(TasksFile, make(chan int)); err == nil {
			t.Fatal("expected an encoding error")
		}

		loaded := store.LoadTasks()
		if len(loaded) != 1 || loaded[0].Title != "Keep me" {
			t.Errorf("previous file was damaged: %+v", loaded)
		}
		if leftovers := tempFiles(t, store.Dir()); len(leftovers) != 0 {
			t.Errorf("failed write left %v behind", leftovers)
		}
	})

	t.Run("an unwritable directory reports an error", func(t *testing.T) {
		blocked := filepath.Join(t.TempDir(), "blocker")
		write(t, blocked, "not a directory")
		store := &Store{dir: filepath.Join(blocked, "sub")}
		if err := store.SaveTasks(nil); err == nil {
			t.Error("expected an error writing under a regular file")
		}
	})

	t.Run("NewStore reports an unusable directory", func(t *testing.T) {
		blocked := filepath.Join(t.TempDir(), "blocker")
		write(t, blocked, "not a directory")
		if _, err := NewStore(filepath.Join(blocked, "sub")); err == nil {
			t.Error("expected an error creating a directory under a file")
		}
	})
}

func TestStoreWritesEmptyArrayNotNull(t *testing.T) {
	// The Python version wrote "[]" for an empty list. Writing "null"
	// instead would still load here but would confuse anything else
	// reading the file.
	store := newTestStore(t)
	if err := store.SaveTasks(nil); err != nil {
		t.Fatalf("SaveTasks: %v", err)
	}
	data, err := os.ReadFile(store.Path(TasksFile))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.TrimSpace(string(data)) != "[]" {
		t.Errorf("wrote %q, want []", data)
	}
}

func TestStoreSeparateFiles(t *testing.T) {
	store := newTestStore(t)
	if err := store.SaveSessions(Sessions{NewSession(time.Now(), time.Now(), 25, nil, nil)}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTasks(Tasks{mustTask(t, "T", 1, "")}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveHabits(Habits{mustHabit(t, "H", nil)}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveProjects(Projects{mustProject(t, "P")}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{SessionsFile, TasksFile, HabitsFile, ProjectsFile} {
		if _, err := os.Stat(store.Path(name)); err != nil {
			t.Errorf("%s was not written: %v", name, err)
		}
	}
	if n := len(store.LoadHabits()); n != 1 {
		t.Errorf("habits = %d, want 1", n)
	}
	if n := len(store.LoadProjects()); n != 1 {
		t.Errorf("projects = %d, want 1", n)
	}
}

func TestStoreDiscardsInvalidRecords(t *testing.T) {
	t.Run("tasks without ids", func(t *testing.T) {
		store := newTestStore(t)
		write(t, store.Path(TasksFile),
			`[{"title":"no id"},{"id":"abc","title":"valid"}]`)
		loaded := store.LoadTasks()
		if len(loaded) != 1 || loaded[0].Title != "valid" {
			t.Errorf("loaded %+v, want only the valid record", loaded)
		}
	})

	t.Run("nulls in the array", func(t *testing.T) {
		store := newTestStore(t)
		write(t, store.Path(HabitsFile), `[null,{"id":"a","name":"Real"}]`)
		if got := store.LoadHabits(); len(got) != 1 {
			t.Errorf("loaded %d habits, want 1", len(got))
		}
	})
}

func TestStoreLoadsPythonFiles(t *testing.T) {
	// A file written by the Python implementation, byte for byte. This
	// is the compatibility guarantee that lets someone upgrade without
	// losing their history.
	store := newTestStore(t)
	write(t, store.Path(TasksFile), `[
  {
    "id": "a1b2c3d4e5f6",
    "title": "Write the report",
    "done": false,
    "estimate": 3,
    "completed": 1,
    "created": "2026-09-02T08:55:00",
    "completed_at": null,
    "project_id": "b2c3d4e5f6a1"
  }
]`)
	tasks := store.LoadTasks()
	if len(tasks) != 1 {
		t.Fatalf("loaded %d tasks, want 1", len(tasks))
	}
	task := tasks[0]
	if task.Title != "Write the report" || task.Estimate != 3 ||
		task.Completed != 1 || task.ProjectID != "b2c3d4e5f6a1" {
		t.Errorf("fields did not survive: %+v", task)
	}

	write(t, store.Path(HabitsFile), `[
  {
    "id": "c3d4e5f6a1b2",
    "name": "Meditate",
    "days": [0, 1, 2, 3, 4],
    "completions": ["2026-09-01", "2026-09-02"],
    "created": "2026-08-01"
  }
]`)
	habits := store.LoadHabits()
	if len(habits) != 1 || habits[0].Name != "Meditate" ||
		len(habits[0].Completions) != 2 {
		t.Fatalf("habit did not survive: %+v", habits)
	}
	if habits[0].Created != NewDate(2026, time.August, 1) {
		t.Errorf("Created = %v, want 2026-08-01", habits[0].Created)
	}
}

func TestMigrateLegacy(t *testing.T) {
	legacy := `[{"start":"2026-08-01T09:00:00","end":"2026-08-01T09:25:00","work_minutes":25}]`

	t.Run("imports an old history file", func(t *testing.T) {
		home := t.TempDir()
		write(t, filepath.Join(home, LegacySessionsFile), legacy)
		store := newTestStore(t)

		migrated, err := store.MigrateLegacy(home)
		if err != nil {
			t.Fatalf("MigrateLegacy: %v", err)
		}
		if !migrated {
			t.Fatal("expected a migration to happen")
		}
		if got := store.LoadSessions(); len(got) != 1 {
			t.Errorf("loaded %d sessions after migration, want 1", len(got))
		}
	})

	t.Run("never overwrites existing data", func(t *testing.T) {
		home := t.TempDir()
		write(t, filepath.Join(home, LegacySessionsFile), legacy)
		store := newTestStore(t)
		current := Sessions{NewSession(time.Now(), time.Now(), 50, nil, nil)}
		if err := store.SaveSessions(current); err != nil {
			t.Fatal(err)
		}

		migrated, err := store.MigrateLegacy(home)
		if err != nil || migrated {
			t.Fatalf("migrated = %v, err = %v; want false, nil", migrated, err)
		}
		if got := store.LoadSessions(); got[0].WorkMinutes != 50 {
			t.Error("existing history was overwritten")
		}
	})

	t.Run("no legacy file is not an error", func(t *testing.T) {
		store := newTestStore(t)
		if migrated, err := store.MigrateLegacy(t.TempDir()); migrated || err != nil {
			t.Errorf("migrated = %v, err = %v; want false, nil", migrated, err)
		}
	})

	t.Run("an empty or corrupt legacy file is skipped", func(t *testing.T) {
		for _, content := range []string{"[]", "{{{ garbage"} {
			home := t.TempDir()
			write(t, filepath.Join(home, LegacySessionsFile), content)
			store := newTestStore(t)
			migrated, err := store.MigrateLegacy(home)
			if migrated || err != nil {
				t.Errorf("content %q: migrated = %v, err = %v", content, migrated, err)
			}
			if _, err := os.Stat(store.Path(SessionsFile)); err == nil {
				t.Errorf("content %q: should not have created a file", content)
			}
		}
	})

	t.Run("an unreadable target reports an error", func(t *testing.T) {
		// A path whose parent is a regular file cannot be stat'ed
		// normally, which exercises the non-NotExist branch.
		blocked := filepath.Join(t.TempDir(), "blocker")
		write(t, blocked, "not a directory")
		store := &Store{dir: filepath.Join(blocked, "sub")}
		if _, err := store.MigrateLegacy(t.TempDir()); err == nil {
			t.Error("expected an error checking for existing history")
		}
	})
}

func TestSessions(t *testing.T) {
	project := mustProject(t, "Website")
	task := mustTask(t, "Design", 3, project.ID)
	other := mustTask(t, "Unrelated", 1, "")

	start := time.Date(2026, time.September, 2, 9, 0, 0, 0, time.Local)
	sessions := Sessions{
		NewSession(start, start.Add(25*time.Minute), 25, task, project),
		NewSession(start.Add(time.Hour), start.Add(90*time.Minute), 50, task, project),
		NewSession(start.AddDate(0, 0, -1), start.AddDate(0, 0, -1), 25, other, nil),
	}

	t.Run("daily summary counts only the given day", func(t *testing.T) {
		count, minutes := sessions.DailySummary(fixedToday)
		if count != 2 || minutes != 75 {
			t.Errorf("got (%d, %d), want (2, 75)", count, minutes)
		}
	})

	t.Run("a day with no sessions", func(t *testing.T) {
		count, minutes := sessions.DailySummary(NewDate(2026, time.January, 1))
		if count != 0 || minutes != 0 {
			t.Errorf("got (%d, %d), want (0, 0)", count, minutes)
		}
	})

	t.Run("filtering by task and project", func(t *testing.T) {
		if n := len(sessions.ForTask(task.ID)); n != 2 {
			t.Errorf("ForTask = %d, want 2", n)
		}
		if n := len(sessions.ForProject(project.ID)); n != 2 {
			t.Errorf("ForProject = %d, want 2", n)
		}
		if got := sessions.ProjectMinutes(project.ID); got != 75 {
			t.Errorf("ProjectMinutes = %d, want 75", got)
		}
	})

	t.Run("names are denormalised so history survives deletion", func(t *testing.T) {
		if sessions[0].TaskTitle != "Design" || sessions[0].ProjectName != "Website" {
			t.Errorf("session did not capture names: %+v", sessions[0])
		}
	})

	t.Run("an unattributed session records empty ids", func(t *testing.T) {
		s := NewSession(start, start, 25, nil, nil)
		if s.TaskID != "" || s.ProjectID != "" {
			t.Errorf("expected empty attribution, got %+v", s)
		}
	})
}

func TestNewTask(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		task := mustTask(t, "  Write tests  ", 0, "")
		if task.Title != "Write tests" {
			t.Errorf("Title = %q, want the trimmed form", task.Title)
		}
		if task.Estimate != 1 {
			t.Errorf("Estimate = %d, want the floor of 1", task.Estimate)
		}
		if task.Done || task.Completed != 0 || task.CompletedAt != nil {
			t.Errorf("new task should be untouched: %+v", task)
		}
		if task.ID == "" {
			t.Error("task needs an id")
		}
	})

	t.Run("rejects a blank title", func(t *testing.T) {
		for _, bad := range []string{"", "   ", "\t\n"} {
			if _, err := NewTask(bad, 1, ""); !errors.Is(err, ErrEmptyName) {
				t.Errorf("NewTask(%q) error = %v, want ErrEmptyName", bad, err)
			}
		}
	})

	t.Run("ids are unique", func(t *testing.T) {
		seen := map[string]bool{}
		for i := 0; i < 1000; i++ {
			id := newID()
			if seen[id] {
				t.Fatalf("duplicate id %q after %d draws", id, i)
			}
			seen[id] = true
		}
	})
}

func TestTaskOperations(t *testing.T) {
	newList := func(t *testing.T) (Tasks, *Task, *Task, *Task) {
		t.Helper()
		var tasks Tasks
		a, _ := tasks.Add("Task A", 2, "")
		b, _ := tasks.Add("Task B", 3, "")
		c, _ := tasks.Add("Task C", 1, "")
		return tasks, a, b, c
	}

	t.Run("Add appends in order", func(t *testing.T) {
		tasks, _, _, _ := newList(t)
		if got := titles(tasks); !equalStrings(got, []string{"Task A", "Task B", "Task C"}) {
			t.Errorf("order = %v", got)
		}
	})

	t.Run("Add rejects a blank title", func(t *testing.T) {
		var tasks Tasks
		if _, err := tasks.Add("  ", 1, ""); err == nil {
			t.Error("expected an error")
		}
		if len(tasks) != 0 {
			t.Error("a rejected task should not be appended")
		}
	})

	t.Run("Find", func(t *testing.T) {
		tasks, _, b, _ := newList(t)
		got, err := tasks.Find(b.ID)
		if err != nil || got != b {
			t.Errorf("Find = %v, %v", got, err)
		}
		if _, err := tasks.Find("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("Find(unknown) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("SetDone stamps and clears the completion time", func(t *testing.T) {
		_, a, _, _ := newList(t)
		a.SetDone(true)
		if !a.Done || a.CompletedAt == nil {
			t.Error("completing should stamp the time")
		}
		a.SetDone(false)
		if a.Done || a.CompletedAt != nil {
			t.Error("reopening should clear the time")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		tasks, _, b, _ := newList(t)
		if !tasks.Delete(b.ID) {
			t.Error("Delete should report success")
		}
		if got := titles(tasks); !equalStrings(got, []string{"Task A", "Task C"}) {
			t.Errorf("after delete = %v", got)
		}
		if tasks.Delete("nope") {
			t.Error("deleting an unknown id should report false")
		}
	})

	t.Run("Move reorders and clamps at both ends", func(t *testing.T) {
		tasks, a, _, c := newList(t)

		if _, err := tasks.Move(c.ID, -1); err != nil {
			t.Fatal(err)
		}
		if got := titles(tasks); !equalStrings(got, []string{"Task A", "Task C", "Task B"}) {
			t.Errorf("after move up = %v", got)
		}
		if _, err := tasks.Move(c.ID, 1); err != nil {
			t.Fatal(err)
		}
		if got := titles(tasks); !equalStrings(got, []string{"Task A", "Task B", "Task C"}) {
			t.Errorf("after move down = %v", got)
		}

		// Moving past an end must clamp, never wrap.
		if idx, _ := tasks.Move(a.ID, -1); idx != 0 {
			t.Errorf("clamped index = %d, want 0", idx)
		}
		if got := titles(tasks)[0]; got != "Task A" {
			t.Errorf("first task = %q, want Task A", got)
		}
		if idx, _ := tasks.Move(c.ID, 1); idx != 2 {
			t.Errorf("clamped index = %d, want 2", idx)
		}
		if _, err := tasks.Move("nope", 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("Move(unknown) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Move by more than one position", func(t *testing.T) {
		tasks, a, _, _ := newList(t)
		if idx, _ := tasks.Move(a.ID, 2); idx != 2 {
			t.Errorf("index = %d, want 2", idx)
		}
		if got := titles(tasks); !equalStrings(got, []string{"Task B", "Task C", "Task A"}) {
			t.Errorf("after move = %v", got)
		}
	})

	t.Run("ClearCompleted removes only finished tasks", func(t *testing.T) {
		tasks, a, _, c := newList(t)
		a.SetDone(true)
		c.SetDone(true)
		if got := tasks.ClearCompleted(); got != 2 {
			t.Errorf("removed %d, want 2", got)
		}
		if got := titles(tasks); !equalStrings(got, []string{"Task B"}) {
			t.Errorf("remaining = %v", got)
		}
	})

	t.Run("Open excludes finished tasks", func(t *testing.T) {
		tasks, a, _, _ := newList(t)
		a.SetDone(true)
		if got := titles(tasks.Open()); !equalStrings(got, []string{"Task B", "Task C"}) {
			t.Errorf("open = %v", got)
		}
	})

	t.Run("TaskStats", func(t *testing.T) {
		tasks, a, _, c := newList(t)
		a.Completed = 1 // one of two
		c.SetDone(true)
		open, done, remaining := tasks.TaskStats()
		if open != 2 || done != 1 || remaining != 1+3 {
			t.Errorf("got (%d, %d, %d), want (2, 1, 4)", open, done, remaining)
		}
	})

	t.Run("overshooting an estimate never goes negative", func(t *testing.T) {
		tasks, a, _, _ := newList(t)
		a.Completed = 10 // estimate is 2
		if got := a.Remaining(); got != 0 {
			t.Errorf("Remaining = %d, want 0", got)
		}
		_, _, remaining := tasks.TaskStats()
		if remaining != 3+1 {
			t.Errorf("remaining = %d, want 4", remaining)
		}
	})

	t.Run("ProgressLabel", func(t *testing.T) {
		_, a, _, _ := newList(t)
		a.Completed = 1
		if got := a.ProgressLabel(); got != "1/2" {
			t.Errorf("ProgressLabel = %q, want 1/2", got)
		}
	})
}

func TestTaskNormalize(t *testing.T) {
	// Guards against files written by an older version, or hand-edited.
	task := &Task{}
	task.normalize()
	if task.ID == "" || task.Title != "Untitled" || task.Estimate != 1 {
		t.Errorf("normalize left gaps: %+v", task)
	}
	if task.Created.Time().IsZero() {
		t.Error("normalize should supply a creation time")
	}

	negative := &Task{ID: "x", Title: "T", Estimate: -5, Completed: -3}
	negative.normalize()
	if negative.Estimate != 1 || negative.Completed != 0 {
		t.Errorf("normalize did not repair negatives: %+v", negative)
	}
}

func TestNewIDFallback(t *testing.T) {
	// crypto/rand is documented never to fail on supported platforms,
	// but the fallback must still work: an application should not
	// panic over an identifier.
	original := randRead
	randRead = func([]byte) (int, error) { return 0, errors.New("no entropy") }
	defer func() { randRead = original }()

	first, second := newID(), newID()
	if first == "" || first == second {
		t.Errorf("fallback ids should be non-empty and distinct: %q, %q", first, second)
	}
	if !strings.HasPrefix(first, "fallback") {
		t.Errorf("id = %q, want the fallback form", first)
	}
}

// --- helpers ---------------------------------------------------------

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func tempFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp_") {
			out = append(out, e.Name())
		}
	}
	return out
}

func mustTask(t *testing.T, title string, estimate int, projectID string) *Task {
	t.Helper()
	task, err := NewTask(title, estimate, projectID)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	return task
}

func mustHabit(t *testing.T, name string, days []int) *Habit {
	t.Helper()
	habit, err := NewHabit(name, days, fixedToday)
	if err != nil {
		t.Fatalf("NewHabit: %v", err)
	}
	return habit
}

func mustProject(t *testing.T, name string) *Project {
	t.Helper()
	project, err := NewProject(name, "", Date{}, fixedToday)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	return project
}

func titles(tasks Tasks) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Title
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestWriteJSONFailurePaths exercises the failure branches of the
// atomic write. Real files cannot be made to fail these ways inside a
// test — especially running as root, where permission tricks are
// ignored — so the two filesystem calls are injected instead.
func TestWriteJSONFailurePaths(t *testing.T) {
	restore := func() {
		createTemp = os.CreateTemp
		renameFile = os.Rename
	}

	t.Run("the temp file cannot be created", func(t *testing.T) {
		defer restore()
		createTemp = func(string, string) (*os.File, error) {
			return nil, errors.New("no space left on device")
		}
		store := newTestStore(t)
		if err := store.SaveTasks(nil); err == nil {
			t.Error("expected an error")
		}
	})

	t.Run("the write fails and the temp file is cleaned up", func(t *testing.T) {
		defer restore()
		var tmpPath string
		createTemp = func(dir, pattern string) (*os.File, error) {
			f, err := os.CreateTemp(dir, pattern)
			if err != nil {
				return nil, err
			}
			tmpPath = f.Name()
			// A closed handle makes the next Write fail.
			f.Close()
			return f, nil
		}
		store := newTestStore(t)
		if err := store.SaveTasks(nil); err == nil {
			t.Error("expected an error")
		}
		if _, err := os.Stat(tmpPath); err == nil {
			t.Error("the temp file should have been removed")
		}
	})

	t.Run("the rename fails and the temp file is cleaned up", func(t *testing.T) {
		defer restore()
		renameFile = func(string, string) error {
			return errors.New("cross-device link")
		}
		store := newTestStore(t)
		if err := store.SaveTasks(nil); err == nil {
			t.Error("expected an error")
		}
		if leftovers := tempFiles(t, store.Dir()); len(leftovers) != 0 {
			t.Errorf("failed rename left %v behind", leftovers)
		}
	})

	t.Run("a failed migration reports the write error", func(t *testing.T) {
		defer restore()
		home := t.TempDir()
		write(t, filepath.Join(home, LegacySessionsFile),
			`[{"start":"2026-08-01T09:00:00","end":"2026-08-01T09:25:00","work_minutes":25}]`)
		store := newTestStore(t)
		renameFile = func(string, string) error { return errors.New("disk full") }
		if _, err := store.MigrateLegacy(home); err == nil {
			t.Error("expected the write error to propagate")
		}
	})
}

// TestLoadTolerance covers the decode-failure path for every record
// type, not just sessions.
func TestLoadTolerance(t *testing.T) {
	corrupt := `{"not": "an array"}`

	t.Run("tasks", func(t *testing.T) {
		store := newTestStore(t)
		write(t, store.Path(TasksFile), corrupt)
		if got := store.LoadTasks(); len(got) != 0 {
			t.Errorf("loaded %d tasks, want 0", len(got))
		}
	})
	t.Run("habits", func(t *testing.T) {
		store := newTestStore(t)
		write(t, store.Path(HabitsFile), corrupt)
		if got := store.LoadHabits(); len(got) != 0 {
			t.Errorf("loaded %d habits, want 0", len(got))
		}
	})
	t.Run("projects", func(t *testing.T) {
		store := newTestStore(t)
		write(t, store.Path(ProjectsFile), corrupt)
		if got := store.LoadProjects(); len(got) != 0 {
			t.Errorf("loaded %d projects, want 0", len(got))
		}
	})
	t.Run("a file of only invalid records loads as empty", func(t *testing.T) {
		store := newTestStore(t)
		write(t, store.Path(ProjectsFile), `[{"name":"no id"},{"name":"also no id"}]`)
		if got := store.LoadProjects(); got != nil {
			t.Errorf("loaded %v, want nil", got)
		}
	})
}

func TestWriteJSONSyncFailure(t *testing.T) {
	// Sync on a pipe fails with EINVAL, which is the only way to reach
	// this branch without a filesystem fault injector: a write to the
	// pipe buffer succeeds, and the fsync that follows does not.
	defer func() { createTemp = os.CreateTemp }()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer reader.Close()
	createTemp = func(string, string) (*os.File, error) { return writer, nil }

	store := newTestStore(t)
	err = store.SaveTasks(nil)
	if err == nil {
		t.Fatal("expected a sync error")
	}
	if !strings.Contains(err.Error(), "syncing") {
		t.Errorf("error = %v, want it to mention syncing", err)
	}
}
