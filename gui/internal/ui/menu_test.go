package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestBuildMenu(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := test.NewWindow(widget.NewLabel(""))
	defer w.Close()

	menu := buildMenu(a, w)
	if menu == nil || len(menu.Items) != 1 {
		t.Fatalf("buildMenu() = %+v, want exactly one top-level menu", menu)
	}
	if got, want := menu.Items[0].Label, "File"; got != want {
		t.Errorf("top-level menu label = %q, want %q", got, want)
	}

	// Exactly one Quit item: Fyne auto-appends its own fallback Quit
	// item to the menu bar unless it finds one already labeled exactly
	// "Quit" — a label of "Quit (Ctrl+Q)" produced two Quit entries in
	// the running app, confirmed by screenshot, before this label was
	// fixed. A regression here would silently reintroduce that bug
	// (Fyne's own item has no Go-visible marker to assert against
	// directly, so this pins the condition that avoids it).
	items := menu.Items[0].Items
	if len(items) != 1 {
		t.Fatalf("File menu has %d items, want exactly 1 (Quit): %+v", len(items), items)
	}
	if got, want := items[0].Label, "Quit"; got != want {
		t.Errorf("Quit item label = %q, want %q (must be exact, or Fyne adds its own)", got, want)
	}
	if items[0].Shortcut == nil {
		t.Error("Quit item has no Shortcut set — Fyne won't render or bind the accelerator")
	}
}
