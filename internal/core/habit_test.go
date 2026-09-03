package core

import (
	"errors"
	"testing"
	"time"
)

// habitWith builds a habit with an exact history, for deterministic
// assertions. Created is set far back so it never floors a streak
// unless a test asks for that.
func habitWith(t *testing.T, completions []string, days []int) *Habit {
	t.Helper()
	habit, err := NewHabit("Habit", days, NewDate(2026, time.January, 1))
	if err != nil {
		t.Fatalf("NewHabit: %v", err)
	}
	habit.Completions = append([]string{}, completions...)
	return habit
}

// daysBack returns ISO dates for the n consecutive days ending at end.
func daysBack(n int, end Date) []string {
	out := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		out = append(out, end.AddDays(-i).String())
	}
	return out
}

func TestNormalizeDays(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"deduplicated and sorted", []int{3, 1, 1, 0}, []int{0, 1, 3}},
		{"out-of-range values dropped", []int{9, -1, 2}, []int{2}},
		{"empty falls back to daily", []int{}, Daily},
		{"nil falls back to daily", nil, Daily},
		{"all-invalid falls back to daily", []int{99, -4}, Daily},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeDays(tc.in); !equalInts(got, tc.want) {
				t.Errorf("NormalizeDays(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewHabit(t *testing.T) {
	t.Run("defaults to every day", func(t *testing.T) {
		habit := mustHabit(t, "  Meditate  ", nil)
		if habit.Name != "Meditate" {
			t.Errorf("Name = %q, want the trimmed form", habit.Name)
		}
		if !equalInts(habit.Days, Daily) {
			t.Errorf("Days = %v, want every day", habit.Days)
		}
		if len(habit.Completions) != 0 {
			t.Error("a new habit should have no history")
		}
	})

	t.Run("a habit can never have an empty schedule", func(t *testing.T) {
		// A habit scheduled for no days could never be kept.
		habit := mustHabit(t, "x", []int{})
		if !equalInts(habit.Days, Daily) {
			t.Errorf("Days = %v, want every day", habit.Days)
		}
	})

	t.Run("rejects a blank name", func(t *testing.T) {
		if _, err := NewHabit("   ", nil, fixedToday); !errors.Is(err, ErrEmptyName) {
			t.Errorf("error = %v, want ErrEmptyName", err)
		}
	})

	t.Run("an unset creation date becomes today", func(t *testing.T) {
		habit, err := NewHabit("x", nil, Date{})
		if err != nil {
			t.Fatal(err)
		}
		if habit.Created != Today() {
			t.Errorf("Created = %v, want today", habit.Created)
		}
	})
}

func TestHabitNormalize(t *testing.T) {
	t.Run("fills gaps and repairs history", func(t *testing.T) {
		habit := &Habit{
			ID:          "abc",
			Completions: []string{"2026-09-02", "garbage", "2026-09-01", "2026-09-02"},
		}
		habit.normalize()
		if habit.Name != "Untitled" {
			t.Errorf("Name = %q", habit.Name)
		}
		if !equalInts(habit.Days, Daily) {
			t.Errorf("Days = %v", habit.Days)
		}
		// Unparseable dropped, duplicates removed, result sorted.
		want := []string{"2026-09-01", "2026-09-02"}
		if !equalStrings(habit.Completions, want) {
			t.Errorf("Completions = %v, want %v", habit.Completions, want)
		}
	})

	t.Run("a missing creation date falls back to the first completion", func(t *testing.T) {
		habit := &Habit{ID: "a", Completions: []string{"2026-08-09", "2026-08-05"}}
		habit.normalize()
		if habit.Created != NewDate(2026, time.August, 5) {
			t.Errorf("Created = %v, want the earliest completion", habit.Created)
		}
	})

	t.Run("no history at all falls back to today", func(t *testing.T) {
		habit := &Habit{ID: "a"}
		habit.normalize()
		if habit.Created != Today() {
			t.Errorf("Created = %v, want today", habit.Created)
		}
	})

	t.Run("generates a missing id", func(t *testing.T) {
		habit := &Habit{}
		habit.normalize()
		if habit.ID == "" {
			t.Error("normalize should supply an id")
		}
	})
}

func TestHabitScheduleAndMarking(t *testing.T) {
	habit := mustHabit(t, "Gym", Weekdays)
	saturday := NewDate(2026, time.September, 5)

	t.Run("IsScheduled respects the weekday list", func(t *testing.T) {
		if !habit.IsScheduled(fixedToday) {
			t.Error("Wednesday should be scheduled for a weekdays habit")
		}
		if habit.IsScheduled(saturday) {
			t.Error("Saturday should not be scheduled for a weekdays habit")
		}
	})

	t.Run("Mark and unmark", func(t *testing.T) {
		habit.Mark(fixedToday, true)
		if !habit.IsDone(fixedToday) {
			t.Error("should be done after marking")
		}
		habit.Mark(fixedToday, true) // marking twice must not duplicate
		if len(habit.Completions) != 1 {
			t.Errorf("Completions = %v, want one entry", habit.Completions)
		}
		habit.Mark(fixedToday, false)
		if habit.IsDone(fixedToday) || len(habit.Completions) != 0 {
			t.Errorf("unmarking failed: %v", habit.Completions)
		}
		habit.Mark(fixedToday, false) // unmarking an unmarked day is harmless
	})

	t.Run("Toggle flips both ways", func(t *testing.T) {
		habit.Toggle(fixedToday)
		if !habit.IsDone(fixedToday) {
			t.Error("toggle should mark done")
		}
		habit.Toggle(fixedToday)
		if habit.IsDone(fixedToday) {
			t.Error("toggle should mark undone")
		}
	})

	t.Run("completions stay sorted", func(t *testing.T) {
		h := mustHabit(t, "Sorted", nil)
		for _, d := range []Date{
			NewDate(2026, time.September, 3),
			NewDate(2026, time.September, 1),
			NewDate(2026, time.September, 2),
		} {
			h.Mark(d, true)
		}
		want := []string{"2026-09-01", "2026-09-02", "2026-09-03"}
		if !equalStrings(h.Completions, want) {
			t.Errorf("Completions = %v, want %v", h.Completions, want)
		}
	})
}

func TestHabitStreaks(t *testing.T) {
	tests := []struct {
		name        string
		completions []string
		days        []int
		want        int
	}{
		{
			name:        "unbroken daily streak",
			completions: daysBack(5, fixedToday),
			want:        5,
		},
		{
			name: "no history means no streak",
			want: 0,
		},
		{
			name:        "a gap breaks the streak",
			completions: []string{"2026-08-30", "2026-09-01", "2026-09-02"},
			want:        2,
		},
		{
			// A daily habit not yet done at 9am keeps yesterday's
			// streak; it breaks only once today has passed unkept.
			name:        "today gets grace while still in progress",
			completions: []string{"2026-08-31", "2026-09-01"},
			want:        2,
		},
		{
			name:        "a missed yesterday does break the streak",
			completions: []string{"2026-08-31"},
			want:        0,
		},
		{
			// Missing a Saturday cannot break a weekdays-only streak.
			name:        "unscheduled days are skipped, not counted as misses",
			completions: []string{"2026-08-28", "2026-08-31", "2026-09-01", "2026-09-02"},
			days:        Weekdays,
			want:        4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			habit := habitWith(t, tc.completions, tc.days)
			if got := habit.CurrentStreak(fixedToday); got != tc.want {
				t.Errorf("CurrentStreak = %d, want %d", got, tc.want)
			}
		})
	}

	t.Run("a brand-new habit does not scan back through empty time", func(t *testing.T) {
		habit := mustHabit(t, "New", nil) // created today
		habit.Mark(fixedToday, true)
		if got := habit.CurrentStreak(fixedToday); got != 1 {
			t.Errorf("CurrentStreak = %d, want 1", got)
		}
	})

	t.Run("backfilled history before the created date still counts", func(t *testing.T) {
		// Regression: completions predating creation were once cut off,
		// silently dropping imported or backdated history.
		habit := mustHabit(t, "Backfilled", nil) // created today
		habit.Completions = daysBack(5, fixedToday)
		if got := habit.CurrentStreak(fixedToday); got != 5 {
			t.Errorf("CurrentStreak = %d, want 5", got)
		}
	})

	t.Run("the lookback is bounded", func(t *testing.T) {
		// A hand-edited absurd date must not hang the UI.
		habit := habitWith(t, nil, nil)
		habit.Created = NewDate(1900, time.January, 1)
		if got := habit.CurrentStreak(fixedToday); got != 0 {
			t.Errorf("CurrentStreak = %d, want 0", got)
		}
	})
}

func TestHabitLongestStreak(t *testing.T) {
	tests := []struct {
		name        string
		completions []string
		days        []int
		want        int
	}{
		{"no history", nil, nil, 0},
		{
			name: "finds the best run",
			completions: []string{
				"2026-08-20", "2026-08-21", "2026-08-22", "2026-08-23", // 4
				"2026-09-01", "2026-09-02", // 2
			},
			want: 4,
		},
		{"includes a run still going", daysBack(3, fixedToday), nil, 3},
		{
			// The same grace as CurrentStreak: an unfinished today must
			// not truncate the run just completed.
			name:        "today gets grace",
			completions: []string{"2026-08-31", "2026-09-01"},
			want:        2,
		},
		{
			name:        "ignores future completions",
			completions: []string{"2026-09-01", "2026-09-02", "2026-12-25"},
			want:        2,
		},
		{
			name:        "skips unscheduled days",
			completions: []string{"2026-08-27", "2026-08-28", "2026-08-31", "2026-09-01"},
			days:        Weekdays,
			want:        4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			habit := habitWith(t, tc.completions, tc.days)
			if got := habit.LongestStreak(fixedToday); got != tc.want {
				t.Errorf("LongestStreak = %d, want %d", got, tc.want)
			}
		})
	}

	t.Run("the forward walk is bounded too", func(t *testing.T) {
		habit := habitWith(t, []string{"2026-09-02"}, nil)
		habit.Created = NewDate(1900, time.January, 1)
		// The cap stops the walk long before reaching 2026, so the run
		// is never found. Asserting this documents the bound rather
		// than leaving it as an accident.
		if got := habit.LongestStreak(fixedToday); got != 0 {
			t.Errorf("LongestStreak = %d, want 0 (walk truncated by the cap)", got)
		}
	})
}

func TestHabitCompletionRate(t *testing.T) {
	tests := []struct {
		name        string
		completions []string
		days        []int
		window      int
		want        float64
	}{
		{"all kept", daysBack(10, fixedToday), nil, 10, 1},
		{"half kept", []string{"2026-08-31", "2026-09-02"}, nil, 4, 0.5},
		{
			// A habit should not be marked down for a day still in
			// progress.
			name:        "an unfinished today is excluded",
			completions: []string{"2026-09-01"},
			window:      2,
			want:        1,
		},
		{
			name:        "counts only scheduled days",
			completions: []string{"2026-08-29", "2026-08-30"},
			days:        Weekends,
			window:      7,
			want:        1,
		},
		{"a zero window", daysBack(3, fixedToday), nil, 0, 0},
		{"a negative window", daysBack(3, fixedToday), nil, -5, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			habit := habitWith(t, tc.completions, tc.days)
			if got := habit.CompletionRate(tc.window, fixedToday); got != tc.want {
				t.Errorf("CompletionRate = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("days before the habit existed are excluded", func(t *testing.T) {
		habit := mustHabit(t, "New", nil) // created today
		habit.Mark(fixedToday, true)
		if got := habit.CompletionRate(30, fixedToday); got != 1 {
			t.Errorf("CompletionRate = %v, want 1", got)
		}
	})

	t.Run("no scheduled days in the window is zero, not a divide by zero", func(t *testing.T) {
		habit := mustHabit(t, "Weekend only", Weekends)
		if got := habit.CompletionRate(3, fixedToday); got != 0 {
			t.Errorf("CompletionRate = %v, want 0", got)
		}
	})
}

func TestHabitRecentWindow(t *testing.T) {
	habit := habitWith(t, []string{"2026-09-02"}, Weekdays)
	window := habit.RecentWindow(7, fixedToday)

	if len(window) != 7 {
		t.Fatalf("window length = %d, want 7", len(window))
	}
	if window[0].Day != fixedToday.AddDays(-6) {
		t.Errorf("window starts at %v, want oldest first", window[0].Day)
	}
	if last := window[6]; last.Day != fixedToday || !last.Scheduled || !last.Done {
		t.Errorf("today = %+v, want scheduled and done", last)
	}
	if tuesday := window[5]; !tuesday.Scheduled || tuesday.Done {
		t.Errorf("Tuesday = %+v, want scheduled and missed", tuesday)
	}
	if sunday := window[3]; sunday.Scheduled {
		t.Errorf("Sunday = %+v, want unscheduled", sunday)
	}
}

func TestHabitScheduleLabel(t *testing.T) {
	tests := []struct {
		days []int
		want string
	}{
		{Daily, "Every day"},
		{Weekdays, "Weekdays"},
		{Weekends, "Weekends"},
		{[]int{0, 2, 4}, "Mon, Wed, Fri"},
		{[]int{6}, "Sun"},
	}
	for _, tc := range tests {
		habit := mustHabit(t, "x", tc.days)
		if got := habit.ScheduleLabel(); got != tc.want {
			t.Errorf("ScheduleLabel(%v) = %q, want %q", tc.days, got, tc.want)
		}
	}
}

func TestHabitCollection(t *testing.T) {
	var habits Habits
	a, _ := habits.Add("Meditate", Daily, fixedToday)
	b, _ := habits.Add("Gym", Weekdays, fixedToday)
	c, _ := habits.Add("Read", nil, fixedToday)

	t.Run("Add rejects a blank name", func(t *testing.T) {
		if _, err := habits.Add("  ", nil, fixedToday); err == nil {
			t.Error("expected an error")
		}
		if len(habits) != 3 {
			t.Error("a rejected habit should not be appended")
		}
	})

	t.Run("Find", func(t *testing.T) {
		if got, err := habits.Find(b.ID); err != nil || got != b {
			t.Errorf("Find = %v, %v", got, err)
		}
		if _, err := habits.Find("nope"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Due and Stats", func(t *testing.T) {
		a.Mark(fixedToday, true)
		// c is daily and unkept; b is a weekday habit, and today is a
		// Wednesday, so it counts too.
		due := habits.Due(fixedToday)
		if len(due) != 2 {
			t.Errorf("Due = %d habits, want 2", len(due))
		}
		scheduled, completed := habits.Stats(fixedToday)
		if scheduled != 3 || completed != 1 {
			t.Errorf("Stats = (%d, %d), want (3, 1)", scheduled, completed)
		}
	})

	t.Run("Stats ignores habits not scheduled today", func(t *testing.T) {
		weekend := Habits{mustHabit(t, "Weekend", Weekends)}
		scheduled, completed := weekend.Stats(fixedToday)
		if scheduled != 0 || completed != 0 {
			t.Errorf("Stats = (%d, %d), want (0, 0)", scheduled, completed)
		}
	})

	t.Run("Move and Delete", func(t *testing.T) {
		if _, err := habits.Move(c.ID, -1); err != nil {
			t.Fatal(err)
		}
		if habits[1] != c {
			t.Error("Move did not reorder")
		}
		if _, err := habits.Move("nope", 1); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
		if !habits.Delete(a.ID) {
			t.Error("Delete should report success")
		}
		if habits.Delete("nope") {
			t.Error("deleting an unknown id should report false")
		}
		if len(habits) != 2 {
			t.Errorf("len = %d, want 2", len(habits))
		}
	})
}

func TestEqualIntsMismatches(t *testing.T) {
	// ScheduleLabel relies on this; a wrong answer would mislabel every
	// custom schedule.
	tests := []struct {
		name string
		a, b []int
		want bool
	}{
		{"identical", []int{0, 1}, []int{0, 1}, true},
		{"different lengths", []int{0}, []int{0, 1}, false},
		{"same length, different values", []int{1, 2}, []int{1, 3}, false},
		{"both empty", nil, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := equalInts(tc.a, tc.b); got != tc.want {
				t.Errorf("equalInts(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
