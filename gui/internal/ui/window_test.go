package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/christopherwc/productivio/gui/internal/app"
)

func TestNewWindow(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	env := &app.Env{Now: func() time.Time { return time.Now() }}
	w := NewWindow(a, env)
	defer w.Close()

	if got, want := w.Title(), "Pomodoro"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if w.Content() == nil {
		t.Error("Content() = nil, want a non-nil root widget")
	}
}
