package core

import (
	"encoding/json"
	"testing"
	"time"
)

// fixedToday is a Wednesday. Every test involving the calendar pins to
// it so the suite cannot behave differently depending on the day it
// happens to run.
var fixedToday = NewDate(2026, time.September, 2)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    string
	}{
		{"standard pomodoro", 25 * 60, "25:00"},
		{"exhausted timer", 0, "00:00"},
		{"single digits are padded", 61, "01:01"},
		{"seconds are padded", 9, "00:09"},
		{"negatives clamp; the clock never goes below zero", -5, "00:00"},
		{"long blocks stay in minutes, not H:MM:SS", 90 * 60, "90:00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatTime(tc.seconds); got != tc.want {
				t.Errorf("FormatTime(%d) = %q, want %q", tc.seconds, got, tc.want)
			}
		})
	}
}

func TestFormatMinutes(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, "0m"}, {45, "45m"}, {60, "1h"}, {90, "1h 30m"},
		{720, "12h"}, {-5, "0m"},
	}
	for _, tc := range tests {
		if got := FormatMinutes(tc.minutes); got != tc.want {
			t.Errorf("FormatMinutes(%d) = %q, want %q", tc.minutes, got, tc.want)
		}
	}
}

func TestPercentLabel(t *testing.T) {
	tests := []struct {
		fraction float64
		want     string
	}{
		{0, "0%"},
		{0.333, "33%"},
		{1, "100%"},
		{5, "500%"}, // effort overrun is not capped
		// Go rounds half away from zero where Python used banker's
		// rounding, so this shows 13% here and 12% there. Asserted so
		// nobody "corrects" it back by accident.
		{0.125, "13%"},
	}
	for _, tc := range tests {
		if got := PercentLabel(tc.fraction); got != tc.want {
			t.Errorf("PercentLabel(%v) = %q, want %q", tc.fraction, got, tc.want)
		}
	}
}

func TestDateArithmetic(t *testing.T) {
	t.Run("AddDays crosses month boundaries", func(t *testing.T) {
		got := fixedToday.AddDays(-3)
		if want := NewDate(2026, time.August, 30); got != want {
			t.Errorf("AddDays(-3) = %v, want %v", got, want)
		}
	})

	t.Run("DaysUntil is signed", func(t *testing.T) {
		if got := fixedToday.DaysUntil(NewDate(2026, time.September, 30)); got != 28 {
			t.Errorf("DaysUntil forward = %d, want 28", got)
		}
		if got := fixedToday.DaysUntil(NewDate(2026, time.August, 30)); got != -3 {
			t.Errorf("DaysUntil backward = %d, want -3", got)
		}
	})

	t.Run("WeekdayIndex uses Monday as zero", func(t *testing.T) {
		// Go's time.Weekday puts Sunday at 0; the stored schedules use
		// Python's Monday-first convention. Confusing the two would
		// silently shift every habit schedule by a day.
		tests := []struct {
			date Date
			want int
			name string
		}{
			{NewDate(2026, time.August, 31), 0, "Monday"},
			{fixedToday, 2, "Wednesday"},
			{NewDate(2026, time.September, 5), 5, "Saturday"},
			{NewDate(2026, time.September, 6), 6, "Sunday"},
		}
		for _, tc := range tests {
			if got := tc.date.WeekdayIndex(); got != tc.want {
				t.Errorf("%s WeekdayIndex = %d, want %d", tc.name, got, tc.want)
			}
		}
	})

	t.Run("zero date is the unset marker", func(t *testing.T) {
		if !(Date{}).IsZero() {
			t.Error("zero Date should report IsZero")
		}
		if fixedToday.IsZero() {
			t.Error("a real date should not report IsZero")
		}
	})

	t.Run("Today returns the real current day", func(t *testing.T) {
		if got := Today(); got != DateOf(time.Now()) {
			t.Errorf("Today() = %v, want the current date", got)
		}
	})
}

func TestParseDate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := ParseDate("2026-09-02")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != fixedToday {
			t.Errorf("ParseDate = %v, want %v", got, fixedToday)
		}
	})
	for _, bad := range []string{"", "garbage", "2026-13-45", "02/09/2026"} {
		t.Run("rejects "+bad, func(t *testing.T) {
			if _, err := ParseDate(bad); err == nil {
				t.Errorf("ParseDate(%q) should have failed", bad)
			}
		})
	}
}

// TestPythonWireFormat is the compatibility guarantee: files written by
// the Python implementation must load here, and files written here must
// look the same going back out. Breaking this would strand every
// existing installation's history.
func TestPythonWireFormat(t *testing.T) {
	t.Run("timestamp has no zone offset", func(t *testing.T) {
		ts := Timestamp(time.Date(2026, time.September, 2, 9, 0, 0, 0, time.Local))
		data, err := json.Marshal(ts)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if want := `"2026-09-02T09:00:00"`; string(data) != want {
			t.Errorf("marshalled %s, want %s", data, want)
		}
	})

	t.Run("timestamp round-trips", func(t *testing.T) {
		var ts Timestamp
		if err := json.Unmarshal([]byte(`"2026-09-02T09:25:00"`), &ts); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got := ts.String(); got != "2026-09-02T09:25:00" {
			t.Errorf("round-tripped to %q", got)
		}
	})

	t.Run("timestamp tolerates RFC 3339 from another tool", func(t *testing.T) {
		var ts Timestamp
		if err := json.Unmarshal([]byte(`"2026-09-02T09:25:00Z"`), &ts); err != nil {
			t.Errorf("should accept RFC 3339: %v", err)
		}
	})

	t.Run("timestamp rejects nonsense", func(t *testing.T) {
		var ts Timestamp
		if err := json.Unmarshal([]byte(`"not a time"`), &ts); err == nil {
			t.Error("expected an error for an unparseable timestamp")
		}
		if err := json.Unmarshal([]byte(`12345`), &ts); err == nil {
			t.Error("expected an error for a non-string timestamp")
		}
	})

	t.Run("unset date marshals to null", func(t *testing.T) {
		data, err := json.Marshal(Date{})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(data) != "null" {
			t.Errorf("marshalled %s, want null", data)
		}
	})

	t.Run("date round-trips", func(t *testing.T) {
		data, err := json.Marshal(fixedToday)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if want := `"2026-09-02"`; string(data) != want {
			t.Errorf("marshalled %s, want %s", data, want)
		}
	})

	t.Run("a hand-edited bad date degrades to unset", func(t *testing.T) {
		// These files are user-editable. A bad date should mean "not
		// set" rather than making the whole file unreadable.
		for _, raw := range []string{`"garbage"`, `null`, `42`, `true`} {
			var d Date
			if err := json.Unmarshal([]byte(raw), &d); err != nil {
				t.Errorf("Unmarshal(%s) errored: %v", raw, err)
			}
			if !d.IsZero() {
				t.Errorf("Unmarshal(%s) = %v, want the zero date", raw, d)
			}
		}
	})

	t.Run("malformed json still reports an error", func(t *testing.T) {
		var d Date
		if err := json.Unmarshal([]byte(`{`), &d); err == nil {
			t.Error("expected an error for malformed JSON")
		}
	})
}

func TestDateUnmarshalMalformed(t *testing.T) {
	// Called directly, because malformed JSON inside a document fails
	// in the outer decoder before this method is ever reached.
	var d Date
	if err := d.UnmarshalJSON([]byte("{")); err == nil {
		t.Error("expected an error for malformed JSON")
	}
}
