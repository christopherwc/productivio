// Package app provides the persistence and environment abstraction the
// GUI's views are built against.
//
// This mirrors internal/cli's Store/Env pattern deliberately: an
// explicit environment rather than reaching for globals, so views can
// be tested without touching real files or a real clock. See
// internal/cli.Store's doc comment for why the interface is declared
// here, at the consumer, rather than importing *core.Store directly.
package app

import (
	"time"

	"github.com/christopherwc/productivio/internal/core"
	"github.com/christopherwc/productivio/internal/platform"
)

// Store is the persistence surface the GUI uses.
type Store interface {
	Dir() string
	LoadSessions() core.Sessions
	SaveSessions(core.Sessions) error
	LoadTasks() core.Tasks
	SaveTasks(core.Tasks) error
	LoadHabits() core.Habits
	SaveHabits(core.Habits) error
	LoadProjects() core.Projects
	SaveProjects(core.Projects) error
}

// Env is everything a view needs from the outside world.
type Env struct {
	Store Store

	// Now returns the current instant. Pinned in tests.
	Now func() time.Time

	// Notify raises a desktop notification.
	Notify func(title, message string) bool

	// AfterFunc schedules f to run once after d, returning a Timer
	// that can be stopped. The pomodoro timer (a later commit) is the
	// only user; tests substitute a fake clock so they don't sleep.
	AfterFunc func(d time.Duration, f func()) *time.Timer
}

// Today is the current calendar day according to the environment.
func (e *Env) Today() core.Date { return core.DateOf(e.Now()) }

// NewRealEnv wires up the real environment: the platform data
// directory, a persistent Store, one-time legacy-data migration, and
// the real clock/notification/timer implementations.
//
// migrated reports whether pre-existing Python-version session
// history was imported on this call, so the caller can surface that
// to the user once.
func NewRealEnv() (env *Env, migrated bool, err error) {
	dir, err := platform.DataDir()
	if err != nil {
		return nil, false, err
	}
	store, err := core.NewStore(dir)
	if err != nil {
		return nil, false, err
	}
	if home, herr := platform.HomeDir(); herr == nil {
		migrated, _ = store.MigrateLegacy(home)
	}
	return &Env{
		Store:     store,
		Now:       time.Now,
		Notify:    platform.Notify,
		AfterFunc: time.AfterFunc,
	}, migrated, nil
}
