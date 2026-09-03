// Package ui builds the Fyne widget tree for the Pomodoro GUI.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/christopherwc/productivio/gui/internal/app"
)

// NewWindow builds the main application window: a tab per view. Later
// commits add tabs here for tasks, projects and habits; they append to
// this container rather than rebuilding it.
func NewWindow(a fyne.App, env *app.Env) fyne.Window {
	w := a.NewWindow("Pomodoro")
	tabs := container.NewAppTabs(
		container.NewTabItem("Dashboard", NewDashboard(env)),
		container.NewTabItem("Tasks", NewTasksTab(env)),
	)
	w.SetContent(tabs)
	w.Resize(fyne.NewSize(480, 320))
	return w
}
