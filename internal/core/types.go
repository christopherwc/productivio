// Package core holds the data model and persistence for the Pomodoro
// application: sessions, tasks, habits and projects.
//
// It has no user-interface code and no external dependencies, so the
// whole model is testable without a display, exactly as the Python
// original's core module was.
//
// # File-format compatibility
//
// The JSON written here is byte-compatible with the Python version, so
// an existing installation's sessions.json, tasks.json, habits.json and
// projects.json load unchanged. That constrains the time encoding:
// Go's time.Time marshals to RFC 3339 with an offset ("2026-09-02T09:00:00Z"),
// whereas Python wrote naive local timestamps ("2026-09-02T09:00:00").
// The Timestamp and Date types below exist to preserve the original
// wire format rather than silently break every existing data file.
//
// # Design differences from the Python original
//
// Persistence is a Store value carrying its own directory rather than
// module-level functions reading a global. Dependency injection is the
// idiomatic Go approach and removes the need for tests to monkey-patch
// a package-level function, which is what the Python suite had to do.
package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Layouts matching what the Python implementation wrote.
const (
	timestampLayout = "2006-01-02T15:04:05" // isoformat(timespec="seconds")
	dateLayout      = "2006-01-02"          // date.isoformat()
)

// Timestamp is a wall-clock instant with no zone, encoded the way
// Python's datetime.isoformat(timespec="seconds") writes it.
type Timestamp time.Time

// MarshalJSON renders the timestamp without a zone offset.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(timestampLayout))
}

// UnmarshalJSON accepts the naive layout, and tolerates a full RFC 3339
// value so a file touched by another tool still loads.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, layout := range []string{timestampLayout, time.RFC3339} {
		if parsed, err := time.Parse(layout, s); err == nil {
			*t = Timestamp(parsed)
			return nil
		}
	}
	return fmt.Errorf("core: cannot parse timestamp %q", s)
}

// Time converts back to the standard library type.
func (t Timestamp) Time() time.Time { return time.Time(t) }

// String renders the stored layout, which is also what the UI shows.
func (t Timestamp) String() string { return time.Time(t).Format(timestampLayout) }

// Date is a calendar day with no time or zone component. Habits and
// project deadlines are day-granular; using time.Time directly would
// invite zone bugs where a completion recorded at 23:30 lands on the
// wrong day after a conversion.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate builds a Date from its parts.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// DateOf truncates an instant to the calendar day it falls on, in that
// instant's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Today is the current local calendar day.
//
// Every function that needs "today" takes it as a parameter instead of
// calling this, so tests can pin the date and never depend on the day
// they happen to run.
func Today() Date { return DateOf(time.Now()) }

// ParseDate reads an ISO "YYYY-MM-DD" string.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("core: cannot parse date %q", s)
	}
	return DateOf(t), nil
}

// String renders the ISO form.
func (d Date) String() string { return d.time().Format(dateLayout) }

// IsZero reports whether the date is unset, which is how "no deadline"
// is represented.
func (d Date) IsZero() bool { return d == Date{} }

func (d Date) time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// AddDays returns the date `n` days later; negative values go back.
func (d Date) AddDays(n int) Date { return DateOf(d.time().AddDate(0, 0, n)) }

// Before reports whether d falls before other.
func (d Date) Before(other Date) bool { return d.time().Before(other.time()) }

// After reports whether d falls after other.
func (d Date) After(other Date) bool { return d.time().After(other.time()) }

// Weekday returns the day of the week. Habit schedules are stored with
// Monday as 0, so use WeekdayIndex for that conversion.
func (d Date) Weekday() time.Weekday { return d.time().Weekday() }

// WeekdayIndex returns the weekday with Monday as 0 through Sunday as 6.
//
// Go's time.Weekday puts Sunday at 0; the stored habit schedules use
// Python's convention, so the two must not be confused. Keeping the
// conversion in one named function is what stops that happening.
func (d Date) WeekdayIndex() int { return (int(d.Weekday()) + 6) % 7 }

// DaysUntil counts whole days from d to other; negative if other is past.
func (d Date) DaysUntil(other Date) int {
	return int(other.time().Sub(d.time()).Hours() / 24)
}

// MarshalJSON renders the ISO form, or null when unset.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts an ISO string, or null for an unset date.
//
// An unparseable value yields the zero Date rather than an error: these
// files can be hand-edited, and a bad date should degrade to "not set"
// instead of making the whole file unreadable.
func (d *Date) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s, ok := raw.(string)
	if !ok {
		*d = Date{}
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		*d = Date{}
		return nil
	}
	*d = parsed
	return nil
}

// FormatTime renders a second count as MM:SS for the countdown display.
//
// Negative values clamp to zero so the clock can never show something
// like "-1:59" if a tick overshoots.
func FormatTime(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// FormatMinutes renders a minute count compactly: 45m, 1h 30m, 12h.
func FormatMinutes(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	hours, rem := minutes/60, minutes%60
	switch {
	case hours == 0:
		return fmt.Sprintf("%dm", rem)
	case rem == 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dh %dm", hours, rem)
	}
}

// PercentLabel renders a 0.0-1.0 fraction as a rounded percentage.
//
// Go's math.Round rounds half away from zero, unlike Python's banker's
// rounding, so 12.5% shows as 13% here and 12% there. The difference is
// cosmetic but real, and it is asserted in the tests so nobody
// "corrects" it back by accident.
func PercentLabel(fraction float64) string {
	return fmt.Sprintf("%d%%", int(fraction*100+0.5))
}

// cleanTitle trims a user-supplied name and reports whether anything
// is left. Shared by tasks, habits and projects, all of which reject
// blank names.
func cleanTitle(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	return trimmed, trimmed != ""
}
