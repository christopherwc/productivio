package core

import (
	"testing"
	"time"
)

var (
	mergeOlder = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	mergeNewer = mergeOlder.Add(time.Hour)
)

func TestMergeTasks(t *testing.T) {
	t.Run("local-only and remote-only are both kept, local's order first", func(t *testing.T) {
		local := Tasks{{ID: "a", Title: "A", UpdatedAt: mergeOlder}}
		remote := Tasks{{ID: "b", Title: "B", UpdatedAt: mergeOlder}}
		merged := MergeTasks(local, remote)
		if len(merged) != 2 || merged[0].ID != "a" || merged[1].ID != "b" {
			t.Fatalf("merged = %+v", merged)
		}
	})

	t.Run("newer local wins when both sides have the record", func(t *testing.T) {
		local := Tasks{{ID: "a", Title: "Local", UpdatedAt: mergeNewer}}
		remote := Tasks{{ID: "a", Title: "Remote", UpdatedAt: mergeOlder}}
		merged := MergeTasks(local, remote)
		if merged[0].Title != "Local" {
			t.Errorf("title = %q, want Local", merged[0].Title)
		}
	})

	t.Run("newer remote wins when both sides have the record", func(t *testing.T) {
		local := Tasks{{ID: "a", Title: "Local", UpdatedAt: mergeOlder}}
		remote := Tasks{{ID: "a", Title: "Remote", UpdatedAt: mergeNewer}}
		merged := MergeTasks(local, remote)
		if merged[0].Title != "Remote" {
			t.Errorf("title = %q, want Remote", merged[0].Title)
		}
	})

	t.Run("an exact tie prefers local", func(t *testing.T) {
		local := Tasks{{ID: "a", Title: "Local", UpdatedAt: mergeOlder}}
		remote := Tasks{{ID: "a", Title: "Remote", UpdatedAt: mergeOlder}}
		merged := MergeTasks(local, remote)
		if merged[0].Title != "Local" {
			t.Errorf("title = %q, want Local", merged[0].Title)
		}
	})

	t.Run("a newer delete on one side propagates as a tombstone", func(t *testing.T) {
		deletedAt := mergeNewer
		local := Tasks{{ID: "a", Title: "A", UpdatedAt: mergeNewer, DeletedAt: &deletedAt}}
		remote := Tasks{{ID: "a", Title: "A", UpdatedAt: mergeOlder}}
		merged := MergeTasks(local, remote)
		if merged[0].DeletedAt == nil {
			t.Error("the newer tombstone should win the merge")
		}
	})
}

func TestMergeHabits(t *testing.T) {
	local := Habits{{ID: "a", Name: "Local", UpdatedAt: mergeOlder}}
	remote := Habits{
		{ID: "a", Name: "Remote", UpdatedAt: mergeNewer},
		{ID: "b", Name: "New", UpdatedAt: mergeOlder},
	}
	merged := MergeHabits(local, remote)
	if len(merged) != 2 || merged[0].Name != "Remote" || merged[1].Name != "New" {
		t.Fatalf("merged = %+v", merged)
	}
}

func TestMergeProjectsClearsParentPointingToATombstonedProject(t *testing.T) {
	// Device A deletes "Parent" (p1). Device B, offline at the time,
	// independently filed a new project "Child" (p2) under p1 and
	// never saw the deletion. After merging A's and B's state, p2's
	// ParentID points at a now-tombstoned project — repairHierarchy
	// must clear it, the same way it clears a dangling reference.
	deletedAt := mergeNewer
	local := Projects{
		{ID: "p1", Name: "Parent", UpdatedAt: mergeNewer, DeletedAt: &deletedAt},
	}
	remote := Projects{
		{ID: "p1", Name: "Parent", UpdatedAt: mergeOlder},
		{ID: "p2", Name: "Child", ParentID: "p1", UpdatedAt: mergeOlder},
	}

	merged := MergeProjects(local, remote)
	child, err := Projects(merged).find("p2")
	if err != nil {
		t.Fatalf("child project missing from merge: %v", err)
	}
	if child.ParentID != "" {
		t.Errorf("ParentID = %q, want cleared since its parent is tombstoned", child.ParentID)
	}
}

// find is a test-only lookup that, unlike Find, does not hide a
// tombstoned record — needed here because the parent itself (p1) is
// expected to still be present, just deleted.
func (ps Projects) find(id string) (*Project, error) {
	for _, p := range ps {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func TestMergeSessions(t *testing.T) {
	early := Timestamp(time.Date(2026, time.January, 1, 9, 0, 0, 0, time.UTC))
	late := Timestamp(time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC))

	local := Sessions{{Start: late, WorkMinutes: 25}}
	remote := Sessions{
		{Start: early, WorkMinutes: 25},
		{Start: late, WorkMinutes: 25}, // duplicate of local's own session
	}

	merged := MergeSessions(local, remote)
	if len(merged) != 2 {
		t.Fatalf("len = %d, want 2 (the duplicate should be deduped)", len(merged))
	}
	if !merged[0].Start.Time().Equal(early.Time()) {
		t.Errorf("first session = %v, want the earlier one", merged[0].Start)
	}
}
