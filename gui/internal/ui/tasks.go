package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// taskRowText is the single-line label for one task row: title,
// project (if any), progress, and due date (if any, flagged when
// overdue). Kept separate from the widget wiring below so it can be
// tested without a Fyne driver.
func taskRowText(t *core.Task, projects core.Projects, today core.Date) string {
	parts := []string{t.Title}
	if t.ProjectID != "" {
		parts = append(parts, projects.NameOf(t.ProjectID, "-"))
	}
	parts = append(parts, t.ProgressLabel())
	if t.Priority != core.PriorityNone {
		parts = append(parts, t.Priority.String())
	}
	if !t.Due.IsZero() {
		due := "due " + t.Due.String()
		if t.IsOverdue(today) {
			due += " (overdue)"
		}
		parts = append(parts, due)
	}
	if len(t.Tags) > 0 {
		parts = append(parts, core.FormatTags(t.Tags))
	}
	return strings.Join(parts, "  ·  ")
}

// filteredTasks applies the priority and tag filters the Tasks tab's
// controls offer, mirroring the CLI's `task list [priority]` and
// `task list -tag`. "All" is the un-filtered state, distinct from
// filtering to exactly PriorityNone tasks, which "None" names
// explicitly. Kept separate from the widget wiring below so it can be
// tested without a Fyne driver.
func filteredTasks(tasks core.Tasks, priority, tag string) core.Tasks {
	filtered := tasks
	switch priority {
	case "None":
		filtered = filtered.WithPriority(core.PriorityNone)
	case "Low":
		filtered = filtered.WithPriority(core.PriorityLow)
	case "Medium":
		filtered = filtered.WithPriority(core.PriorityMedium)
	case "High":
		filtered = filtered.WithPriority(core.PriorityHigh)
	}
	if tag = strings.TrimSpace(tag); tag != "" {
		filtered = filtered.WithTag(tag)
	}
	return filtered
}

// Row indices into the container newTaskRow builds, so updateItem can
// find each child without a custom widget type.
const (
	taskRowCheck = iota
	taskRowLabel
	taskRowUp
	taskRowDown
	taskRowDelete
)

func newTaskRow() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewCheck("", nil),
		widget.NewLabel(""),
		widget.NewButton("↑", nil),
		widget.NewButton("↓", nil),
		widget.NewButton("✕", nil),
	)
}

// NewTasksTab builds the task list view: add, complete, reorder,
// delete and bulk-clear-completed tasks, each optionally filed under a
// project.
//
// Fyne has no built-in drag-to-reorder or swipe-to-delete outside its
// mobile build, so reordering is explicit up/down buttons and deletion
// an explicit button, matching the rest of this view's plain-buttons
// style.
func NewTasksTab(env *app.Env) fyne.CanvasObject {
	var tasks core.Tasks
	var projects core.Projects
	var shown core.Tasks

	var list *widget.List
	var refresh func()
	var applyFilters func()

	// "All" is the un-filtered state, distinct from filtering to
	// exactly PriorityNone tasks, which "None" names explicitly.
	priorityFilter := widget.NewSelect([]string{"All", "None", "Low", "Medium", "High"}, nil)
	priorityFilter.SetSelected("All")
	tagFilter := widget.NewEntry()
	tagFilter.SetPlaceHolder("Filter by tag")

	applyFilters = func() {
		shown = filteredTasks(tasks, priorityFilter.Selected, tagFilter.Text)
		list.Refresh()
	}
	priorityFilter.OnChanged = func(string) { applyFilters() }
	tagFilter.OnChanged = func(string) { applyFilters() }

	save := func() {
		_ = env.Store.SaveTasks(tasks)
		refresh()
	}
	refresh = func() {
		tasks = env.Store.LoadTasks()
		projects = env.Store.LoadProjects()
		applyFilters()
	}
	updateRow := func(id widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fyne.Container).Objects
		task := shown[id]

		check := row[taskRowCheck].(*widget.Check)
		check.Checked = task.Done
		check.OnChanged = func(bool) {
			task.SetDone(!task.Done)
			save()
		}
		check.Refresh()

		row[taskRowLabel].(*widget.Label).SetText(taskRowText(task, projects, env.Today()))

		row[taskRowUp].(*widget.Button).OnTapped = func() {
			_, _ = tasks.Move(task.ID, -1)
			save()
		}
		row[taskRowDown].(*widget.Button).OnTapped = func() {
			_, _ = tasks.Move(task.ID, 1)
			save()
		}
		row[taskRowDelete].(*widget.Button).OnTapped = func() {
			tasks.Delete(task.ID)
			save()
		}
	}

	list = widget.NewList(func() int { return len(shown) }, newTaskRow, updateRow)
	refresh()

	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("New task")
	estimateEntry := widget.NewEntry()
	estimateEntry.SetPlaceHolder("Pomodoros")
	dueEntry := widget.NewEntry()
	dueEntry.SetPlaceHolder("Due (YYYY-MM-DD)")

	projectNames := []string{"-"}
	for _, p := range projects {
		projectNames = append(projectNames, p.Name)
	}
	projectSelect := widget.NewSelect(projectNames, nil)
	projectSelect.SetSelected("-")

	addButton := widget.NewButton("Add", func() {
		if titleEntry.Text == "" {
			return
		}
		estimate := 1
		if v, err := strconv.Atoi(estimateEntry.Text); err == nil {
			estimate = v
		}
		projectID := ""
		if projectSelect.Selected != "-" {
			for _, p := range projects {
				if p.Name == projectSelect.Selected {
					projectID = p.ID
					break
				}
			}
		}
		var due core.Date
		if dueEntry.Text != "" {
			parsed, err := core.ParseDate(dueEntry.Text)
			if err != nil {
				return
			}
			due = parsed
		}
		task, err := tasks.Add(titleEntry.Text, estimate, projectID)
		if err != nil {
			return
		}
		task.Due = due
		titleEntry.SetText("")
		estimateEntry.SetText("")
		dueEntry.SetText("")
		projectSelect.SetSelected("-")
		save()
	})

	// See the due-entry width below: an Entry's minimum size is barely
	// wider than its cursor — it does not grow to fit its placeholder
	// text — so every secondary field is wrapped at a fixed width and
	// the title field is given the Border's stretching center slot,
	// rather than all of them being packed into an HBox at their
	// unusably narrow natural size.
	trailing := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(90, estimateEntry.MinSize().Height), estimateEntry),
		projectSelect,
		container.NewGridWrap(fyne.NewSize(140, dueEntry.MinSize().Height), dueEntry),
		addButton)
	addForm := container.NewBorder(nil, nil, nil, trailing, titleEntry)

	filterRow := container.NewHBox(
		widget.NewLabel("Priority:"),
		priorityFilter,
		container.NewGridWrap(fyne.NewSize(140, tagFilter.MinSize().Height), tagFilter))

	clearButton := widget.NewButton("Clear done", func() {
		tasks.ClearCompleted()
		save()
	})

	top := container.NewVBox(addForm, filterRow)
	return container.NewBorder(top, clearButton, nil, nil, list)
}
