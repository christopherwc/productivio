package core

import (
	"sort"
	"time"
)

// mergeByID combines two versions of an ID-keyed, timestamped
// collection: a record present on only one side is kept as-is
// (including a tombstone, so a deletion propagates the next time the
// other side pulls), and a record present on both sides keeps whichever
// copy has the later UpdatedAt, local winning an exact tie. The result
// keeps local's order and appends any remote-only records in remote's
// order, matching how Add already appends new records — so, unlike
// content edits, a Move on one device does not itself sync (see
// Tasks.Move).
func mergeByID[T any](local, remote []T, id func(T) string, updatedAt func(T) time.Time) []T {
	remoteByID := make(map[string]T, len(remote))
	for _, r := range remote {
		remoteByID[id(r)] = r
	}

	seen := make(map[string]bool, len(local))
	out := make([]T, 0, len(local)+len(remote))
	for _, l := range local {
		seen[id(l)] = true
		if r, ok := remoteByID[id(l)]; ok && updatedAt(r).After(updatedAt(l)) {
			out = append(out, r)
			continue
		}
		out = append(out, l)
	}
	for _, r := range remote {
		if !seen[id(r)] {
			out = append(out, r)
		}
	}
	return out
}

// MergeTasks combines a local and a remote task list, keyed by id.
func MergeTasks(local, remote Tasks) Tasks {
	return mergeByID(local, remote,
		func(t *Task) string { return t.ID },
		func(t *Task) time.Time { return t.UpdatedAt },
	)
}

// MergeHabits combines a local and a remote habit list, keyed by id.
func MergeHabits(local, remote Habits) Habits {
	return mergeByID(local, remote,
		func(h *Habit) string { return h.ID },
		func(h *Habit) time.Time { return h.UpdatedAt },
	)
}

// MergeProjects combines a local and a remote project list, keyed by
// id, then repairs the hierarchy: two devices independently reparenting
// or deleting projects can otherwise leave a ParentID that is dangling
// or closes a cycle, which repairHierarchy already guards against for a
// single hand-edited file.
func MergeProjects(local, remote Projects) Projects {
	merged := Projects(mergeByID(local, remote,
		func(p *Project) string { return p.ID },
		func(p *Project) time.Time { return p.UpdatedAt },
	))
	merged.repairHierarchy()
	return merged
}

// sessionKey identifies a session by the facts recorded about it rather
// than an id — Session has none, since it is meant to be an immutable,
// append-only fact, and giving it one now would be a wire-format change
// this merge does not need.
type sessionKey struct {
	Start       Timestamp
	TaskID      string
	ProjectID   string
	WorkMinutes int
}

func keyOf(s *Session) sessionKey {
	return sessionKey{s.Start, s.TaskID, s.ProjectID, s.WorkMinutes}
}

// MergeSessions unions a local and a remote session history, deduping
// by sessionKey, and keeps the result sorted oldest-first to match
// Sessions' documented invariant.
func MergeSessions(local, remote Sessions) Sessions {
	seen := make(map[sessionKey]bool, len(local))
	out := make(Sessions, 0, len(local)+len(remote))
	for _, s := range local {
		seen[keyOf(s)] = true
		out = append(out, s)
	}
	for _, s := range remote {
		k := keyOf(s)
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Time().Before(out[j].Start.Time()) })
	return out
}
