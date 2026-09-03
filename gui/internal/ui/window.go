// Package ui builds the Fyne widget tree for the Pomodoro GUI.
package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/internal/core"
	"github.com/christopherwc/productivio/internal/platform"
)

// NewWindow builds the main application window. Content is a
// placeholder for now; real views land in later commits.
func NewWindow(a fyne.App) fyne.Window {
	w := a.NewWindow("Pomodoro")
	w.SetContent(widget.NewLabel(fmt.Sprintf(
		"Pomodoro GUI — today is %s. Press %s to quit.",
		core.Today(), platform.Accelerator("Q"),
	)))
	w.Resize(fyne.NewSize(480, 320))
	return w
}
