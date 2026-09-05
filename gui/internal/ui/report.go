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

// reportRowText is the single-line label for one project's totals
// within a report window. Kept separate from the widget wiring below
// so it can be tested without a Fyne driver.
func reportRowText(t core.ProjectTotal) string {
	name := t.ProjectName
	if name == "" {
		name = "-"
	}
	noun := "sessions"
	if t.Sessions == 1 {
		noun = "session"
	}
	return fmt.Sprintf("%s  ·  %d %s  ·  %s", name, t.Sessions, noun, core.FormatMinutes(t.Minutes))
}

// reportSummaryText is the header line above the per-project rows: how
// many pomodoros were logged in the window and their total focus time.
func reportSummaryText(days int, totals []core.ProjectTotal) string {
	sessionCount, minutes := 0, 0
	for _, t := range totals {
		sessionCount += t.Sessions
		minutes += t.Minutes
	}
	noun := "days"
	if days == 1 {
		noun = "day"
	}
	return fmt.Sprintf("Last %d %s: %d pomodoros (%s)", days, noun, sessionCount, core.FormatMinutes(minutes))
}

// NewReportTab builds the focus-time-by-project view: the same
// per-project breakdown `pomodoro report` prints on the command line,
// over a day window the field above the list controls (default 7,
// matching the CLI).
func NewReportTab(env *app.Env) fyne.CanvasObject {
	var totals []core.ProjectTotal

	var list *widget.List
	var summary *widget.Label
	var refresh func()

	daysEntry := widget.NewEntry()
	daysEntry.SetText("7")

	refresh = func() {
		days := 7
		if v, err := strconv.Atoi(daysEntry.Text); err == nil && v > 0 {
			days = v
		}
		since := env.Today().AddDays(-(days - 1))
		totals = env.Store.LoadSessions().Report(since)
		summary.SetText(reportSummaryText(days, totals))
		list.Refresh()
	}

	summary = widget.NewLabel("")
	list = widget.NewList(
		func() int { return len(totals) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(reportRowText(totals[id]))
		},
	)
	refresh()

	refreshButton := widget.NewButton("Refresh", refresh)
	controls := container.NewHBox(
		widget.NewLabel("Days:"),
		container.NewGridWrap(fyne.NewSize(60, daysEntry.MinSize().Height), daysEntry),
		refreshButton)

	top := container.NewVBox(controls, summary)
	return container.NewBorder(top, nil, nil, nil, list)
}
