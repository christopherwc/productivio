package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestNewWindow(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	w := NewWindow(a)
	defer w.Close()

	if got, want := w.Title(), "Pomodoro"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if w.Content() == nil {
		t.Error("Content() = nil, want a non-nil root widget")
	}
}
