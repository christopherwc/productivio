package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// buildMenu returns the application's main menu bar and registers its
// keyboard shortcuts on w.
//
// The Quit item's label must be exactly "Quit": Fyne auto-appends its
// own fallback Quit item to the menu bar when it doesn't find one
// with that exact label already present, which produced two Quit
// entries when this label instead read "Quit (Ctrl+Q)" — confirmed by
// screenshot before settling on this. Its Shortcut field is what
// makes Fyne render the platform-correct accelerator next to the
// label automatically (Cmd+Q on macOS, Ctrl+Q elsewhere) — the same
// distinction internal/platform's own Modifier/Accelerator helpers
// exist to make for the CLI's text output, just via Fyne's mechanism
// for the GUI instead of a hand-built string.
func buildMenu(a fyne.App, w fyne.Window) *fyne.MainMenu {
	shortcut := &desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierShortcutDefault}

	quit := fyne.NewMenuItem("Quit", func() { a.Quit() })
	quit.Shortcut = shortcut
	w.Canvas().AddShortcut(shortcut, func(fyne.Shortcut) { a.Quit() })

	return fyne.NewMainMenu(fyne.NewMenu("File", quit))
}
