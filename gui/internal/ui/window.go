// Package ui builds the Fyne widget tree for the Pomodoro GUI.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/christopherwc/productivio/gui/internal/app"
)

// NewWindow builds the main application window: a tab per view.
func NewWindow(a fyne.App, env *app.Env) fyne.Window {
	w := a.NewWindow("Pomodoro")
	tabs := container.NewAppTabs(
		container.NewTabItem("Dashboard", NewDashboard(env)),
		container.NewTabItem("Tasks", NewTasksTab(env)),
		container.NewTabItem("Projects", NewProjectsTab(env)),
		container.NewTabItem("Habits", NewHabitsTab(env)),
		container.NewTabItem("Timer", NewTimerTab(env)),
		container.NewTabItem("Report", NewReportTab(env)),
	)
	w.SetContent(tabs)
	w.SetMainMenu(buildMenu(a, w))
	// Wide enough that a task/habit row's up/down/delete buttons stay
	// on screen, and tall enough to show several rows without
	// scrolling immediately. Verified against a real render, not
	// guessed: 480x320 (the scaffold's placeholder size) clipped
	// buttons off the right edge of every list view.
	w.Resize(fyne.NewSize(820, 560))
	return w
}
