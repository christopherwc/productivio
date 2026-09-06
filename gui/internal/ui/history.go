package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// historyRowText is the single-line label for one session, in the same
// form `pomodoro history` prints: day, start time, minutes and the
// task/project it was credited to. Kept separate from the widget
// wiring below so it can be tested without a Fyne driver.
func historyRowText(s *core.Session) string {
	task := s.TaskTitle
	if task == "" {
		task = "-"
	}
	project := s.ProjectName
	if project == "" {
		project = "-"
	}
	return fmt.Sprintf("%s  %s  ·  %s  ·  %s · %s",
		s.Day().String(), s.Start.Time().Format("15:04"),
		core.FormatMinutes(s.WorkMinutes), task, project)
}

// NewHistoryTab builds the session-log view: the same recent-sessions
// list `pomodoro history` prints on the command line, newest first,
// over a count the field above the list controls (default 10,
// matching the CLI).
func NewHistoryTab(env *app.Env) fyne.CanvasObject {
	var sessions core.Sessions

	var list *widget.List
	var refresh func()

	limitEntry := widget.NewEntry()
	limitEntry.SetText("10")

	refresh = func() {
		limit := 10
		if v, err := strconv.Atoi(limitEntry.Text); err == nil && v > 0 {
			limit = v
		}
		all := env.Store.LoadSessions()
		if limit > len(all) {
			limit = len(all)
		}
		sessions = make(core.Sessions, limit)
		for i := 0; i < limit; i++ {
			// Newest first: the CLI walks the same slice back to front.
			sessions[i] = all[len(all)-1-i]
		}
		list.Refresh()
	}

	list = widget.NewList(
		func() int { return len(sessions) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(historyRowText(sessions[id]))
		},
	)
	refresh()

	refreshButton := widget.NewButton("Refresh", refresh)
	controls := container.NewHBox(
		widget.NewLabel("Show:"),
		container.NewGridWrap(fyne.NewSize(60, limitEntry.MinSize().Height), limitEntry),
		refreshButton)

	return container.NewBorder(controls, nil, nil, nil, list)
}
