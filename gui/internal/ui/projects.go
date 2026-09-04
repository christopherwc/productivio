package ui

import (
	"fmt"
	"image/color"
	"strings"

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

// projectEntry pairs a project with its depth in the hierarchy (0 for
// a top-level project, 1 for its direct subprojects, and so on), so
// the list can indent subprojects under their parent.
type projectEntry struct {
	project *core.Project
	depth   int
}

// flattenProjectTree walks the project hierarchy depth-first, roots
// first, so every subproject is listed immediately after its parent
// rather than in raw storage order.
func flattenProjectTree(projects core.Projects) []projectEntry {
	var out []projectEntry
	var walk func(p *core.Project, depth int)
	walk = func(p *core.Project, depth int) {
		out = append(out, projectEntry{project: p, depth: depth})
		for _, child := range projects.Children(p.ID) {
			walk(child, depth+1)
		}
	}
	for _, root := range projects.Children("") {
		walk(root, 0)
	}
	return out
}

// projectIndent is how far one level of subproject nesting shifts a
// row's label, matching the two-space-per-level convention `pomodoro
// project list` uses on the command line.
const projectIndent = "    "

// NewProjectsTab builds the project list view: list, add (optionally
// under a parent project) and complete projects — the same surface
// `pomodoro project` exposes on the command line. Subprojects are
// shown indented under their parent. (The CLI also supports moving an
// existing project with `project parent`; this view only files a new
// one under a parent at creation, since reparenting needs a control
// this view does not yet have.)
func NewProjectsTab(env *app.Env) fyne.CanvasObject {
	var projects core.Projects
	var tasks core.Tasks
	var sessions core.Sessions
	var entries []projectEntry

	var list *widget.List
	var refresh func()
	parentSelect := widget.NewSelect([]string{"-"}, nil)
	parentSelect.SetSelected("-")

	save := func() {
		_ = env.Store.SaveProjects(projects)
		refresh()
	}
	refresh = func() {
		projects = env.Store.LoadProjects()
		tasks = env.Store.LoadTasks()
		sessions = env.Store.LoadSessions()
		entries = flattenProjectTree(projects)

		parentNames := []string{"-"}
		for _, p := range projects {
			parentNames = append(parentNames, p.Name)
		}
		parentSelect.Options = parentNames
		parentSelect.Refresh()

		list.Refresh()
	}
	updateRow := func(id widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fyne.Container).Objects
		entry := entries[id]
		p := entry.project
		today := env.Today()

		health := row[projectRowHealth].(*canvas.Text)
		health.Text = projects.Health(p, tasks, today)
		health.Color = healthColor(health.Text)
		health.Refresh()

		label := strings.Repeat(projectIndent, entry.depth) + projectRowText(p, projects, tasks, sessions, today)
		row[projectRowLabel].(*widget.Label).SetText(label)

		done := row[projectRowDone].(*widget.Button)
		done.OnTapped = func() {
			p.Complete()
			save()
		}
	}

	list = widget.NewList(func() int { return len(entries) }, newProjectRow, updateRow)
	refresh()

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("New project")
	dueEntry := widget.NewEntry()
	dueEntry.SetPlaceHolder("Due (YYYY-MM-DD)")

	addButton := widget.NewButton("Add", func() {
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
		project, err := projects.Add(nameEntry.Text, "", due, env.Today())
		if err != nil {
			return
		}
		if parentSelect.Selected != "-" {
			for _, p := range projects {
				if p.Name == parentSelect.Selected {
					// Cannot fail: project is brand new, so it cannot
					// already be an ancestor of p.
					_ = projects.SetParent(project.ID, p.ID)
					break
				}
			}
		}
		nameEntry.SetText("")
		dueEntry.SetText("")
		parentSelect.SetSelected("-")
		save()
	})

	// See tasks.go's NewTasksTab for why the secondary fields are
	// wrapped at a fixed width instead of sharing an HBox at their
	// unusably narrow natural size.
	trailing := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(140, dueEntry.MinSize().Height), dueEntry),
		parentSelect, addButton)
	addForm := container.NewBorder(nil, nil, nil, trailing, nameEntry)

	return container.NewBorder(addForm, nil, nil, nil, list)
}
