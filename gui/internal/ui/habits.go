package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// habitRowText is one habit row's label: name, schedule, a 7-day grid
// (# kept, . missed, blank unscheduled), current streak and 30-day
// completion rate — the same columns `pomodoro habit list` prints.
// Kept separate from the widget wiring so it's testable without a
// Fyne driver.
func habitRowText(h *core.Habit, today core.Date) string {
	var grid strings.Builder
	for _, day := range h.RecentWindow(7, today) {
		switch {
		case day.Done:
			grid.WriteByte('#')
		case day.Scheduled:
			grid.WriteByte('.')
		default:
			grid.WriteByte(' ')
		}
	}
	return fmt.Sprintf("%s  ·  %s  ·  [%s]  ·  streak %d  ·  %s (30d)",
		h.Name, h.ScheduleLabel(), grid.String(), h.CurrentStreak(today),
		core.PercentLabel(h.CompletionRate(30, today)))
}

const (
	habitRowCheck = iota
	habitRowLabel
	habitRowUp
	habitRowDown
	habitRowDelete
)

func newHabitRow() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewCheck("", nil),
		widget.NewLabel(""),
		widget.NewButton("↑", nil),
		widget.NewButton("↓", nil),
		widget.NewButton("✕", nil),
	)
}

// NewHabitsTab builds the habits view: add, check off today, reorder
// and delete habits.
func NewHabitsTab(env *app.Env) fyne.CanvasObject {
	var habits core.Habits

	var list *widget.List
	var refresh func()

	save := func() {
		_ = env.Store.SaveHabits(habits)
		refresh()
	}
	refresh = func() {
		habits = env.Store.LoadHabits()
		list.Refresh()
	}
	updateRow := func(id widget.ListItemID, obj fyne.CanvasObject) {
		row := obj.(*fyne.Container).Objects
		h := habits[id]
		today := env.Today()

		check := row[habitRowCheck].(*widget.Check)
		check.Checked = h.IsDone(today)
		check.OnChanged = func(bool) {
			h.Toggle(today)
			save()
		}
		check.Refresh()

		row[habitRowLabel].(*widget.Label).SetText(habitRowText(h, today))

		row[habitRowUp].(*widget.Button).OnTapped = func() {
			habits.Move(h.ID, -1)
			save()
		}
		row[habitRowDown].(*widget.Button).OnTapped = func() {
			habits.Move(h.ID, 1)
			save()
		}
		row[habitRowDelete].(*widget.Button).OnTapped = func() {
			habits.Delete(h.ID)
			save()
		}
	}

	list = widget.NewList(func() int { return len(habits) }, newHabitRow, updateRow)
	refresh()

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("New habit")

	presetNames := make([]string, len(core.SchedulePresets))
	for i, preset := range core.SchedulePresets {
		presetNames[i] = preset.Label
	}
	scheduleSelect := widget.NewSelect(presetNames, nil)
	scheduleSelect.SetSelected(core.SchedulePresets[0].Label)

	addForm := container.NewBorder(nil, nil, nil, widget.NewButton("Add", func() {
		if nameEntry.Text == "" {
			return
		}
		days := core.Daily
		for _, preset := range core.SchedulePresets {
			if preset.Label == scheduleSelect.Selected {
				days = preset.Days
				break
			}
		}
		if _, err := habits.Add(nameEntry.Text, days, env.Today()); err != nil {
			return
		}
		nameEntry.SetText("")
		scheduleSelect.SetSelected(core.SchedulePresets[0].Label)
		save()
	}), container.NewHBox(nameEntry, scheduleSelect))

	return container.NewBorder(addForm, nil, nil, nil, list)
}
