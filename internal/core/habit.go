package core

import (
	"fmt"
	"sort"
)

// Weekday sets, using Monday as 0 through Sunday as 6 — the convention
// the stored files use. Date.WeekdayIndex performs the conversion from
// Go's Sunday-first time.Weekday.
var (
	Daily    = []int{0, 1, 2, 3, 4, 5, 6}
	Weekdays = []int{0, 1, 2, 3, 4}
	Weekends = []int{5, 6}
)

// WeekdayNames are the abbreviations shown in the UI, Monday first.
var WeekdayNames = [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// SchedulePreset is a named weekday set offered in the UI.
type SchedulePreset struct {
	Label string
	Days  []int
}

// SchedulePresets are the shorthands the UI offers, in display order.
var SchedulePresets = []SchedulePreset{
	{"Every day", Daily},
	{"Weekdays", Weekdays},
	{"Weekends", Weekends},
}

// MaxStreakLookbackDays bounds how far a streak search will walk. A
// habit's history normally stops the loop long before this; the cap
// guarantees that a hand-edited file with an absurd date cannot hang
// the UI.
const MaxStreakLookbackDays = 3660 // about ten years

// Habit is a recurring commitment — "meditate daily", "gym on
// weekdays". Unlike a task it is never finished; what matters is how
// consistently it is kept up.
//
// Schedules are one uniform mechanism, a list of weekday numbers, so
// "every day" is simply all seven. A single representation rather than
// special cases for daily/weekly/custom keeps the streak maths to one
// code path.
type Habit struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Days        []int    `json:"days"`
	Completions []string `json:"completions"` // ISO dates, sorted
	Created     Date     `json:"created"`
}

// Habits is the ordered habit list.
type Habits []*Habit

// NewHabit builds a habit. An empty or invalid day set falls back to
// every day, since a habit scheduled for no days could never be kept.
func NewHabit(name string, days []int, created Date) (*Habit, error) {
	clean, ok := cleanTitle(name)
	if !ok {
		return nil, fmt.Errorf("habit: %w", ErrEmptyName)
	}
	if created.IsZero() {
		created = Today()
	}
	return &Habit{
		ID:          newID(),
		Name:        clean,
		Days:        NormalizeDays(days),
		Completions: []string{},
		Created:     created,
	}, nil
}

// NormalizeDays cleans a weekday list: unique, sorted, in range, never
// empty. Out-of-range entries are dropped rather than rejected, so a
// corrupted file degrades to a sensible schedule instead of breaking
// the habits view.
func NormalizeDays(days []int) []int {
	seen := map[int]bool{}
	for _, d := range days {
		if d >= 0 && d <= 6 {
			seen[d] = true
		}
	}
	if len(seen) == 0 {
		out := make([]int, len(Daily))
		copy(out, Daily)
		return out
	}
	out := make([]int, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Ints(out)
	return out
}

// normalize repairs a habit read from disk.
func (h *Habit) normalize() {
	if h.ID == "" {
		h.ID = newID()
	}
	if h.Name == "" {
		h.Name = "Untitled"
	}
	h.Days = NormalizeDays(h.Days)

	// Drop unparseable dates and duplicates, then sort, so the rest of
	// the program can treat Completions as a clean sorted set.
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(h.Completions))
	for _, c := range h.Completions {
		d, err := ParseDate(c)
		if err != nil || seen[d.String()] {
			continue
		}
		seen[d.String()] = true
		cleaned = append(cleaned, d.String())
	}
	sort.Strings(cleaned)
	h.Completions = cleaned

	if h.Created.IsZero() {
		// Fall back to the earliest completion, or today for a habit
		// with no history at all.
		if len(cleaned) > 0 {
			if d, err := ParseDate(cleaned[0]); err == nil {
				h.Created = d
			}
		}
		if h.Created.IsZero() {
			h.Created = Today()
		}
	}
}

// IsScheduled reports whether the habit is meant to be done on a day.
func (h *Habit) IsScheduled(day Date) bool {
	idx := day.WeekdayIndex()
	for _, d := range h.Days {
		if d == idx {
			return true
		}
	}
	return false
}

// IsDone reports whether the habit was marked complete on a day.
func (h *Habit) IsDone(day Date) bool {
	stamp := day.String()
	for _, c := range h.Completions {
		if c == stamp {
			return true
		}
	}
	return false
}

// Mark records or clears a completion for one date, keeping the stored
// list sorted so the file stays readable and comparisons stay stable.
func (h *Habit) Mark(day Date, done bool) {
	stamp := day.String()
	out := make([]string, 0, len(h.Completions)+1)
	for _, c := range h.Completions {
		if c != stamp {
			out = append(out, c)
		}
	}
	if done {
		out = append(out, stamp)
	}
	sort.Strings(out)
	h.Completions = out
}

// Toggle flips the habit's completion state for one date.
func (h *Habit) Toggle(day Date) { h.Mark(day, !h.IsDone(day)) }

// floor is the earliest date a streak walk should consider: the habit's
// creation date, or the first completion if history was backfilled or
// imported from before it.
func (h *Habit) floor() Date {
	f := h.Created
	if len(h.Completions) > 0 {
		if first, err := ParseDate(h.Completions[0]); err == nil && first.Before(f) {
			return first
		}
	}
	return f
}

// CurrentStreak counts consecutive kept scheduled days ending today.
//
// Two deliberate rules:
//
//   - Unscheduled days are skipped, not counted as misses. Missing a
//     Saturday cannot break a weekdays-only streak.
//   - Today is given grace. A daily habit not yet done at 9am still
//     shows yesterday's streak rather than dropping to zero; it breaks
//     only once today has passed unkept.
func (h *Habit) CurrentStreak(today Date) int {
	cursor := today
	if h.IsScheduled(cursor) && !h.IsDone(cursor) {
		cursor = cursor.AddDays(-1)
	}
	floor := h.floor()

	streak := 0
	for i := 0; i < MaxStreakLookbackDays; i++ {
		if cursor.Before(floor) {
			break
		}
		if h.IsScheduled(cursor) {
			if !h.IsDone(cursor) {
				break
			}
			streak++
		}
		cursor = cursor.AddDays(-1)
	}
	return streak
}

// LongestStreak is the longest run of consecutive kept scheduled days
// on record. Today receives the same grace as CurrentStreak, so a run
// just completed is not truncated by a day still in progress.
func (h *Habit) LongestStreak(today Date) int {
	if len(h.Completions) == 0 {
		return 0
	}
	cursor := h.floor()
	longest, current := 0, 0
	for i := 0; i < MaxStreakLookbackDays; i++ {
		if cursor.After(today) {
			break
		}
		if h.IsScheduled(cursor) {
			switch {
			case h.IsDone(cursor):
				current++
				if current > longest {
					longest = current
				}
			case cursor != today:
				current = 0
			}
		}
		cursor = cursor.AddDays(1)
	}
	return longest
}

// CompletionRate is the fraction of scheduled days kept over the last
// `days` days ending today, from 0.0 to 1.0.
//
// Today counts as a scheduled day only once completed, so a habit is
// not marked down first thing in the morning for a day still in
// progress. Days before the habit existed are excluded. Returns 0.0
// when the window contains nothing scheduled, which keeps callers free
// of divide-by-zero handling.
func (h *Habit) CompletionRate(days int, today Date) float64 {
	if days < 0 {
		days = 0
	}
	created := h.Created
	total, kept := 0, 0
	for offset := 0; offset < days; offset++ {
		day := today.AddDays(-offset)
		if day.Before(created) || !h.IsScheduled(day) {
			continue
		}
		done := h.IsDone(day)
		if day == today && !done {
			continue // still in progress; neither credit nor blame
		}
		total++
		if done {
			kept++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(kept) / float64(total)
}

// DayStatus is one cell of the recent-days grid.
type DayStatus struct {
	Day       Date
	Scheduled bool
	Done      bool
}

// RecentWindow reports the status of each of the last `days` days,
// oldest first — exactly what the habits grid needs to draw itself.
func (h *Habit) RecentWindow(days int, today Date) []DayStatus {
	out := make([]DayStatus, 0, days)
	for offset := days - 1; offset >= 0; offset-- {
		day := today.AddDays(-offset)
		out = append(out, DayStatus{day, h.IsScheduled(day), h.IsDone(day)})
	}
	return out
}

// ScheduleLabel describes the schedule, using a preset name where one
// matches and otherwise listing the weekday abbreviations.
func (h *Habit) ScheduleLabel() string {
	for _, preset := range SchedulePresets {
		if equalInts(h.Days, preset.Days) {
			return preset.Label
		}
	}
	names := make([]string, 0, len(h.Days))
	for _, d := range h.Days {
		names = append(names, WeekdayNames[d])
	}
	return joinWith(names, ", ")
}

// Find returns the habit with the given id.
func (hs Habits) Find(id string) (*Habit, error) {
	for _, h := range hs {
		if h.ID == id {
			return h, nil
		}
	}
	return nil, fmt.Errorf("habit %q: %w", id, ErrNotFound)
}

// Add appends a new habit and returns it.
func (hs *Habits) Add(name string, days []int, created Date) (*Habit, error) {
	habit, err := NewHabit(name, days, created)
	if err != nil {
		return nil, err
	}
	*hs = append(*hs, habit)
	return habit, nil
}

// Delete removes a habit and its history, reporting whether it existed.
func (hs *Habits) Delete(id string) bool {
	for i, h := range *hs {
		if h.ID == id {
			*hs = append((*hs)[:i], (*hs)[i+1:]...)
			return true
		}
	}
	return false
}

// Move reorders a habit by delta positions, clamping at the ends.
func (hs Habits) Move(id string, delta int) (int, error) {
	return move(hs, delta, func(i int) bool { return hs[i].ID == id },
		func(from, to int) {
			item := hs[from]
			copyShift(hs, from, to)
			hs[to] = item
		})
}

// Due returns the habits scheduled for a day that are not yet done.
func (hs Habits) Due(day Date) Habits {
	var out Habits
	for _, h := range hs {
		if h.IsScheduled(day) && !h.IsDone(day) {
			out = append(out, h)
		}
	}
	return out
}

// Stats reports how many habits are scheduled for a day and how many of
// those are already kept.
func (hs Habits) Stats(day Date) (scheduled, completed int) {
	for _, h := range hs {
		if !h.IsScheduled(day) {
			continue
		}
		scheduled++
		if h.IsDone(day) {
			completed++
		}
	}
	return scheduled, completed
}

func equalInts(a, b []int) bool {
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

func joinWith(items []string, sep string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += sep
		}
		out += item
	}
	return out
}
