package ui

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/christopherwc/productivio/gui/internal/app"
	"github.com/christopherwc/productivio/internal/core"
)

// NewTimerTab builds the pomodoro timer view: choose work/rest
// lengths and an optional task, then start/pause/resume a countdown.
//
// The task list is loaded once, at construction — like the other
// three tabs, it does not pick up tasks added from the Tasks tab
// afterwards, since Fyne builds every AppTabs item's content up front
// rather than lazily per tab-switch.
func NewTimerTab(env *app.Env) fyne.CanvasObject {
	timer := app.NewTimer(env)
	tasks := env.Store.LoadTasks().Open()

	workEntry := widget.NewEntry()
	workEntry.SetText("25")
	restEntry := widget.NewEntry()
	restEntry.SetText("5")

	taskNames := []string{"-"}
	for _, t := range tasks {
		taskNames = append(taskNames, t.Title)
	}
	taskSelect := widget.NewSelect(taskNames, nil)
	taskSelect.SetSelected("-")

	statusLabel := widget.NewLabel("Idle")
	countdownLabel := widget.NewLabelWithStyle("--:--", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	var startButton, pauseButton *widget.Button
	refreshLabels := func() {
		phase := "Idle"
		switch timer.Phase {
		case app.PhaseWork:
			phase = "Work"
		case app.PhaseRest:
			phase = "Rest"
		}
		if timer.Paused {
			phase += " (paused)"
		}
		statusLabel.SetText(phase)
		countdownLabel.SetText(core.FormatTime(int(timer.Remaining / time.Second)))

		if timer.Phase == app.PhaseIdle {
			startButton.Enable()
			pauseButton.Disable()
		} else {
			startButton.Disable()
			pauseButton.Enable()
		}
	}

	// scheduleTick re-arms itself every second via env.AfterFunc for as
	// long as the timer is running. The raw AfterFunc callback does
	// nothing but hop onto the UI goroutine — every touch of timer
	// state, env.Store and env.Notify happens inside fyne.Do, matching
	// the concurrency rule the rest of this package follows (no
	// Store/UI access from a background goroutine).
	var scheduleTick func()
	scheduleTick = func() {
		env.AfterFunc(time.Second, func() {
			fyne.Do(func() {
				if timer.Phase == app.PhaseIdle {
					return
				}
				timer.Tick()
				refreshLabels()
				if timer.Phase != app.PhaseIdle {
					scheduleTick()
				}
			})
		})
	}

	startButton = widget.NewButton("Start", func() {
		work, rest := 25, 5
		if v, err := strconv.Atoi(workEntry.Text); err == nil && v > 0 {
			work = v
		}
		if v, err := strconv.Atoi(restEntry.Text); err == nil && v > 0 {
			rest = v
		}
		taskID := ""
		if taskSelect.Selected != "-" {
			for _, t := range tasks {
				if t.Title == taskSelect.Selected {
					taskID = t.ID
					break
				}
			}
		}
		timer.Start(time.Duration(work)*time.Minute, time.Duration(rest)*time.Minute, taskID)
		refreshLabels()
		scheduleTick()
	})
	pauseButton = widget.NewButton("Pause", func() {
		timer.TogglePause()
		refreshLabels()
	})
	refreshLabels()

	form := container.NewHBox(
		widget.NewLabel("Work"),
		container.NewGridWrap(fyne.NewSize(50, workEntry.MinSize().Height), workEntry),
		widget.NewLabel("Rest"),
		container.NewGridWrap(fyne.NewSize(50, restEntry.MinSize().Height), restEntry),
		taskSelect,
	)

	return container.NewVBox(form, container.NewHBox(startButton, pauseButton), statusLabel, countdownLabel)
}
