// Command pomodoro-gui is the desktop GUI for the Pomodoro tracker.
//
// It is a separate binary and a separate Go module from the pomodoro
// CLI (see the root README's "How the GUI works" section): Fyne needs
// cgo and a newer minimum Go version, and keeping them apart lets the
// CLI binary stay a dependency-free, cgo-free static binary.
package main

import (
	"fmt"
	"log"
	"os"

	fyneapp "fyne.io/fyne/v2/app"

	gapp "github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/gui/internal/ui"
)

func main() {
	env, migrated, err := gapp.NewRealEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pomodoro-gui:", err)
		os.Exit(1)
	}
	if migrated {
		log.Println("Imported your existing session history.")
	}

	a := fyneapp.New()
	a.Settings().SetTheme(ui.NewTheme())
	w := ui.NewWindow(a, env)
	w.ShowAndRun()
}
