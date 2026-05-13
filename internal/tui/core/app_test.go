package core

import (
	"testing"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
)

// Simulate a modal (e.g. the master password prompt) being opened at
// startup, before anything has ever been focused. The focus stack entry
// captured at that point is nil.
func TestRestoreFocusWithNilSnapshot(t *testing.T) {
	app := &App{
		Application: tview.NewApplication(),
		manager:     manager.NewElementManager(),
		keys:        &config.KeyBindings{},
	}

	modal := tview.NewForm()

	app.SnapshotFocus()

	app.SetFocusOnly(modal)

	app.RestoreFocus()
}

func TestModalOpenedBeforeAnyFocusDoesNotPanic(t *testing.T) {
	app := &App{
		Application: tview.NewApplication(),
		manager:     manager.NewElementManager(),
		keys:        &config.KeyBindings{},
	}
	app.Pages = NewPages(app.manager, app)

	modal := tview.NewForm()
	app.Pages.ShowModal("TestModal", modal, modal, true, true)
	app.Pages.RemoveModalPage("TestModal")
}
