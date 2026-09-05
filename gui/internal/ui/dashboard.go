package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// NewDashboard builds the status/overview view: today's focus time,
// task and habit progress, and portfolio-level project stats — the
// same rollup `pomodoro status` prints on the command line.
func NewDashboard(env *app.Env) fyne.CanvasObject {
	today := env.Today()
	sessions := env.Store.LoadSessions()
	tasks := env.Store.LoadTasks()
	habits := env.Store.LoadHabits()
	projects := env.Store.LoadProjects()

	count, minutes := sessions.DailySummary(today)
	todayLine := widget.NewLabel(fmt.Sprintf(
		"Today: %d pomodoros (%s)   All time: %d",
		count, core.FormatMinutes(minutes), len(sessions)))

	open, done, remaining, overdue := tasks.TaskStats(today)
	tasksText := fmt.Sprintf("Tasks: %d open, %d done, %d pomodoros remaining", open, done, remaining)
	if overdue > 0 {
		tasksText += fmt.Sprintf(", %d overdue", overdue)
	}
	tasksLine := widget.NewLabel(tasksText)

	return container.NewVBox(
		todayLine,
		tasksLine,
		habitsSection(habits, today),
		widget.NewLabel(projectsLine(projects, tasks, today)),
	)
}

// habitsSection reports how many scheduled habits are kept today, and
// lists whichever are still due with their current streak.
func habitsSection(habits core.Habits, today core.Date) fyne.CanvasObject {
	scheduled, completed := habits.Stats(today)
	if scheduled == 0 {
		return widget.NewLabel("Habits: none scheduled today")
	}

	rows := []fyne.CanvasObject{widget.NewLabel(
		fmt.Sprintf("Habits: %d/%d kept today", completed, scheduled))}
	for _, h := range habits.Due(today) {
		rows = append(rows, widget.NewLabel(fmt.Sprintf(
			"    due: %s (streak %d)", h.Name, h.CurrentStreak(today))))
	}
	return container.NewVBox(rows...)
}

// projectsLine summarises the portfolio: how many projects are active
// and completed, and how many need attention (overdue or at risk).
func projectsLine(projects core.Projects, tasks core.Tasks, today core.Date) string {
	active, completed, needAttention := projects.PortfolioStats(tasks, today)
	line := fmt.Sprintf("Projects: %d active, %d completed", active, completed)
	if needAttention > 0 {
		line += fmt.Sprintf(", %d needing attention", needAttention)
	}
	return line
}
