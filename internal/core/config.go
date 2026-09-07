package core

// Config holds persisted defaults for `pomodoro start`, so a user who
// always works in 50/10 blocks does not have to pass -work and -rest
// on every invocation.
type Config struct {
	WorkMinutes int `json:"work_minutes"`
	RestMinutes int `json:"rest_minutes"`
}

// DefaultConfig is what a fresh installation, or one with no config
// file yet, uses — unchanged from the constants `start` has always
// defaulted to.
var DefaultConfig = Config{WorkMinutes: 25, RestMinutes: 5}
