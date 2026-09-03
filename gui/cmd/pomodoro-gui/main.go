// Command pomodoro-gui is the desktop GUI for the Pomodoro tracker.
//
// It is a separate binary and a separate Go module from the pomodoro
// CLI (see the root README's "How the GUI works" section): Fyne needs
// cgo and a newer minimum Go version, and keeping them apart lets the
// CLI binary stay a dependency-free, cgo-free static binary.
package main

import (
	"fyne.io/fyne/v2/app"

	"github.com/christopherwc/productivio/gui/internal/ui"
)

func main() {
	// Environment wiring (data directory, persistent Store, legacy
	// migration) lands here in a follow-up commit; this is the window
	// scaffold only.
	a := app.New()
	w := ui.NewWindow(a)
	w.ShowAndRun()
}
