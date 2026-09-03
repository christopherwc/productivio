package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// idBytes gives 12 hex characters, matching the Python version's
// uuid4().hex[:12]. Plenty to avoid collisions in a personal task list
// while staying readable in the JSON file.
const idBytes = 6

// randRead is crypto/rand.Read, indirected so a test can force the
// failure path. Reading from the system entropy source does not fail in
// practice, but the fallback below must still be exercised.
var randRead = rand.Read

// newID returns a short unique identifier.
func newID() string {
	b := make([]byte, idBytes)
	if _, err := randRead(b); err != nil {
		// crypto/rand is documented never to fail on the supported
		// platforms. If it somehow does, a counter-based id keeps the
		// application usable rather than panicking over an identifier.
		fallbackCounter++
		return fmt.Sprintf("fallback%04x", fallbackCounter)
	}
	return hex.EncodeToString(b)
}

var fallbackCounter int

// move relocates the element matching `match` by delta positions,
// clamping at the ends of the slice rather than wrapping.
//
// Shared by tasks, habits and projects, which all reorder identically.
func move[T any](items []T, delta int, match func(int) bool, place func(from, to int)) (int, error) {
	for i := range items {
		if !match(i) {
			continue
		}
		target := i + delta
		if target < 0 {
			target = 0
		}
		if target > len(items)-1 {
			target = len(items) - 1
		}
		if target != i {
			place(i, target)
		}
		return target, nil
	}
	return -1, ErrNotFound
}

// copyShift slides the elements between from and to by one position,
// leaving index `to` free for the moved element.
func copyShift[T any](items []T, from, to int) {
	if from < to {
		copy(items[from:to], items[from+1:to+1])
	} else {
		copy(items[to+1:from+1], items[to:from])
	}
}
