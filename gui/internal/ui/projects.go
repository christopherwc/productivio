package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// healthColor maps a core project health label to a display color. One
// place for this mapping, so any view that shows health (today just
// the projects list) stays visually consistent with the rest.
func healthColor(health string) color.Color {
	switch health {
	case core.HealthOverdue:
		return color.NRGBA{R: 0xd9, G: 0x3a, B: 0x3a, A: 0xff}
	case core.HealthAtRisk:
		return color.NRGBA{R: 0xd9, G: 0x8f, B: 0x1f, A: 0xff}
	case core.HealthOnTrack:
		return color.NRGBA{R: 0x2e, G: 0x9e, B: 0x4f, A: 0xff}
	case core.HealthCompleted, core.HealthOnHold, core.HealthNotStarted:
		return color.Gray{Y: 0x80}
	default:
		return color.Black
	}
}

// projectRowText is the non-health part of one project row: name,
// task progress and due date. Kept separate from the widget wiring so
// it can be tested without a Fyne driver.
func projectRowText(p *core.Project, projects core.Projects, tasks core.Tasks, sessions core.Sessions, today core.Date) string {
	s := projects.Summarize(p, tasks, sessions, today)
	due := "-"
	if !p.Due.IsZero() {
		due = p.Due.String()
	}
	return fmt.Sprintf("%s  ·  %d/%d tasks  ·  due %s", p.Name, s.TasksDone, s.TasksTotal, due)
}

const (
	projectRowHealth = iota
	projectRowLabel
	projectRowDone
)

func newProjectRow() fyne.CanvasObject {
	return container.NewHBox(
		canvas.NewText("", color.Black),
		widget.NewLabel(""),
		widget.NewButton("Done", nil),
	)
}

// NewProjectsTab builds the project list view: list, add and complete
// projects — the same surface `pomodoro project` exposes on the
// command line. (The CLI has no project delete/hold/reopen commands,
// so neither does this view.)
func NewProjectsTab(env *app.Env) fyne.CanvasObject {
	var projects core.Projects
	var tasks core.Tasks
	var sessions core.Sessions

	var list *widget.List
	var refresh func()

	save := func() {
		_ = env.Store.SaveProjects(projects)
		refresh()
	}
	refresh = func() {
		projects = env.Store.LoadProjects()
		tasks = env.Store.LoadTasks()
		sessions = env.Store.LoadSessions()
		list.Refresh()
	}
	updateRow := func(id widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fyne.Container).Objects
		p := projects[id]
		today := env.Today()

		health := row[projectRowHealth].(*canvas.Text)
		health.Text = projects.Health(p, tasks, today)
		health.Color = healthColor(health.Text)
		health.Refresh()

		row[projectRowLabel].(*widget.Label).SetText(projectRowText(p, projects, tasks, sessions, today))

		done := row[projectRowDone].(*widget.Button)
		done.OnTapped = func() {
			p.Complete()
			save()
		}
	}

	list = widget.NewList(func() int { return len(projects) }, newProjectRow, updateRow)
	refresh()

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("New project")
	dueEntry := widget.NewEntry()
	dueEntry.SetPlaceHolder("Due (YYYY-MM-DD)")

	addForm := container.NewBorder(nil, nil, nil, widget.NewButton("Add", func() {
		if nameEntry.Text == "" {
			return
		}
		var due core.Date
		if dueEntry.Text != "" {
			parsed, err := core.ParseDate(dueEntry.Text)
			if err != nil {
				return
			}
			due = parsed
		}
		if _, err := projects.Add(nameEntry.Text, "", due, env.Today()); err != nil {
			return
		}
		nameEntry.SetText("")
		dueEntry.SetText("")
		save()
	}), container.NewHBox(nameEntry, dueEntry))

	return container.NewBorder(addForm, nil, nil, nil, list)
}
