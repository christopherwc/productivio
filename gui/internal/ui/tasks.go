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

// taskRowText is the single-line label for one task row: title,
// project (if any) and progress. Kept separate from the widget wiring
// below so it can be tested without a Fyne driver.
func taskRowText(t *core.Task, projects core.Projects) string {
	if t.ProjectID == "" {
		return fmt.Sprintf("%s  ·  %s", t.Title, t.ProgressLabel())
	}
	return fmt.Sprintf("%s  ·  %s  ·  %s",
		t.Title, projects.NameOf(t.ProjectID, "-"), t.ProgressLabel())
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

// NewTasksTab builds the task list view: add, complete, reorder and
// delete tasks, each optionally filed under a project.
//
// Fyne has no built-in drag-to-reorder or swipe-to-delete outside its
// mobile build, so reordering is explicit up/down buttons and deletion
// an explicit button, matching the rest of this view's plain-buttons
// style.
func NewTasksTab(env *app.Env) fyne.CanvasObject {
	var tasks core.Tasks
	var projects core.Projects

	var list *widget.List
	var refresh func()

	save := func() {
		_ = env.Store.SaveTasks(tasks)
		refresh()
	}
	refresh = func() {
		tasks = env.Store.LoadTasks()
		projects = env.Store.LoadProjects()
		list.Refresh()
	}
	updateRow := func(id widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fyne.Container).Objects
		task := tasks[id]

		check := row[taskRowCheck].(*widget.Check)
		check.Checked = task.Done
		check.OnChanged = func(bool) {
			task.SetDone(!task.Done)
			save()
		}
		check.Refresh()

		row[taskRowLabel].(*widget.Label).SetText(taskRowText(task, projects))

		row[taskRowUp].(*widget.Button).OnTapped = func() {
			tasks.Move(task.ID, -1)
			save()
		}
		row[taskRowDown].(*widget.Button).OnTapped = func() {
			tasks.Move(task.ID, 1)
			save()
		}
		row[taskRowDelete].(*widget.Button).OnTapped = func() {
			tasks.Delete(task.ID)
			save()
		}
	}

	list = widget.NewList(func() int { return len(tasks) }, newTaskRow, updateRow)
	refresh()

	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("New task")
	estimateEntry := widget.NewEntry()
	estimateEntry.SetPlaceHolder("Pomodoros")

	projectNames := []string{"-"}
	for _, p := range projects {
		projectNames = append(projectNames, p.Name)
	}
	projectSelect := widget.NewSelect(projectNames, nil)
	projectSelect.SetSelected("-")

	addForm := container.NewBorder(nil, nil, nil, widget.NewButton("Add", func() {
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
		if _, err := tasks.Add(titleEntry.Text, estimate, projectID); err != nil {
			return
		}
		titleEntry.SetText("")
		estimateEntry.SetText("")
		projectSelect.SetSelected("-")
		save()
	}), container.NewHBox(titleEntry, estimateEntry, projectSelect))

	return container.NewBorder(addForm, nil, nil, nil, list)
}
