package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Data file names, unchanged from the Python implementation so an
// existing installation's files are picked up as-is.
const (
	SessionsFile = "sessions.json"
	TasksFile    = "tasks.json"
	HabitsFile   = "habits.json"
	ProjectsFile = "projects.json"

	// LegacySessionsFile is the dotfile the earliest Python versions
	// wrote directly into the home directory. MigrateLegacy imports it
	// once so no history is lost on upgrade.
	LegacySessionsFile = ".pomodoro_sessions.json"
)

// Indirections for the two filesystem calls whose failure modes matter
// but cannot be provoked with real files inside a test. Same pattern as
// randRead and userHomeDir elsewhere in the codebase.
var (
	createTemp = os.CreateTemp
	renameFile = os.Rename
)

// Store reads and writes the application's JSON files in one directory.
//
// The directory is a field rather than a package-level global. That is
// the idiomatic Go arrangement and it also means tests simply construct
// a Store over t.TempDir(); the Python suite had to monkey-patch a
// module function to achieve the same isolation.
type Store struct {
	dir string
}

// NewStore returns a Store rooted at dir, creating it if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("core: creating data directory: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Dir reports the directory the store reads and writes.
func (s *Store) Dir() string { return s.dir }

// Path returns the full path of one of the store's files.
func (s *Store) Path(name string) string { return filepath.Join(s.dir, name) }

// readJSON decodes a JSON array from one of the store's files into out.
//
// Every failure mode returns an empty result rather than an error: a
// missing, unreadable, malformed or wrongly-shaped file must not stop
// the application from starting. A corrupted history is a bad day; a
// program that refuses to open is a worse one.
func (s *Store) readJSON(name string, out any) {
	data, err := os.ReadFile(s.Path(name))
	if err != nil {
		return
	}
	// Reset out to its zero value if decoding fails, so a partially
	// decoded slice never leaks through.
	if err := json.Unmarshal(data, out); err != nil {
		clearSlice(out)
	}
}

// writeJSON atomically writes v as indented JSON to one of the store's
// files.
//
// The write goes to a temporary file in the same directory, is flushed
// and fsynced, and is then moved into place with os.Rename, which is
// atomic on both Linux and macOS. A crash or power loss mid-write
// therefore leaves the previous good file intact instead of a
// half-written one. This matters because the session history is meant
// to be kept permanently.
func (s *Store) writeJSON(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("core: encoding %s: %w", name, err)
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("core: creating data directory: %w", err)
	}

	// The temp file must share a filesystem with the target for the
	// rename to be atomic, hence the same directory.
	tmp, err := createTemp(s.dir, ".tmp_*.json")
	if err != nil {
		return fmt.Errorf("core: creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	// From here on, any failure must remove the temp file so a failed
	// save does not litter the data directory.
	cleanup := func(cause error) error {
		tmp.Close()
		os.Remove(tmpName)
		return cause
	}

	if _, err := tmp.Write(data); err != nil {
		return cleanup(fmt.Errorf("core: writing %s: %w", name, err))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("core: syncing %s: %w", name, err))
	}
	// The only statement in this package the tests do not reach. Close
	// can fail only after both the write and the fsync have already
	// succeeded, and no combination of injectable os calls produces
	// that ordering: a handle that fails to close fails to write
	// first. Left in place rather than removed, because dropping the
	// check would silently discard a real (if rare) error.
	if err := tmp.Close(); err != nil {
		return cleanup(fmt.Errorf("core: closing %s: %w", name, err))
	}
	if err := renameFile(tmpName, s.Path(name)); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("core: replacing %s: %w", name, err)
	}
	return nil
}

// clearSlice empties whichever slice pointer it is handed, used when a
// file fails to decode.
func clearSlice(out any) {
	switch v := out.(type) {
	case *Sessions:
		*v = nil
	case *Tasks:
		*v = nil
	case *Habits:
		*v = nil
	case *Projects:
		*v = nil
	}
}

// LoadSessions reads the permanent pomodoro history.
func (s *Store) LoadSessions() Sessions {
	var sessions Sessions
	s.readJSON(SessionsFile, &sessions)
	return sessions
}

// SaveSessions persists the full session history.
func (s *Store) SaveSessions(sessions Sessions) error {
	return s.writeJSON(SessionsFile, nonNil(sessions))
}

// LoadTasks reads the task list, repairing any incomplete records and
// discarding entries with no id.
func (s *Store) LoadTasks() Tasks {
	var tasks Tasks
	s.readJSON(TasksFile, &tasks)
	return normalizeAll(tasks, func(t *Task) bool { return keep(t.ID, t.normalize) })
}

// SaveTasks persists the task list.
func (s *Store) SaveTasks(tasks Tasks) error {
	return s.writeJSON(TasksFile, nonNil(tasks))
}

// LoadHabits reads the habit list, repairing incomplete records.
func (s *Store) LoadHabits() Habits {
	var habits Habits
	s.readJSON(HabitsFile, &habits)
	return normalizeAll(habits, func(h *Habit) bool { return keep(h.ID, h.normalize) })
}

// SaveHabits persists the habit list.
func (s *Store) SaveHabits(habits Habits) error {
	return s.writeJSON(HabitsFile, nonNil(habits))
}

// LoadProjects reads the project list, repairing incomplete records.
func (s *Store) LoadProjects() Projects {
	var projects Projects
	s.readJSON(ProjectsFile, &projects)
	return normalizeAll(projects, func(p *Project) bool { return keep(p.ID, p.normalize) })
}

// SaveProjects persists the project list.
func (s *Store) SaveProjects(projects Projects) error {
	return s.writeJSON(ProjectsFile, nonNil(projects))
}

// keep discards a record with no id, and repairs the ones it keeps.
//
// The id is checked *before* normalising, because normalising would
// generate one and the record would then be indistinguishable from a
// real entry. A record with no id is corrupt, not merely incomplete.
func keep(id string, normalize func()) bool {
	if id == "" {
		return false
	}
	normalize()
	return true
}

// normalizeAll repairs each record and drops the ones keep rejects.
func normalizeAll[T any](items []*T, keep func(*T) bool) []*T {
	out := make([]*T, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if keep(item) {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// nonNil substitutes an empty slice for a nil one so the file contains
// "[]" rather than "null", which is what the Python version wrote and
// what LoadX expects to read back.
func nonNil[T any](items []*T) []*T {
	if items == nil {
		return []*T{}
	}
	return items
}

// MigrateLegacy imports a pre-existing ~/.pomodoro_sessions.json once.
//
// It runs only when the legacy file exists and the new one does not, so
// it can never overwrite newer data. Reports whether a migration
// actually happened.
func (s *Store) MigrateLegacy(homeDir string) (bool, error) {
	if _, err := os.Stat(s.Path(SessionsFile)); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("core: checking for existing history: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(homeDir, LegacySessionsFile))
	if err != nil {
		return false, nil // nothing to migrate is not an error
	}
	var legacy Sessions
	if err := json.Unmarshal(data, &legacy); err != nil || len(legacy) == 0 {
		return false, nil
	}
	if err := s.SaveSessions(legacy); err != nil {
		return false, err
	}
	return true, nil
}
