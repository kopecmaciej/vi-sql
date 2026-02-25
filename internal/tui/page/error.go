package page

import (
	"fmt"

	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

// showError is a package-local helper that displays an error modal.
// It lives here to avoid a circular dependency on the modal package
// during the bootstrap phase. Once the modal package is built, these
// call sites should migrate to modal.ShowError.
func showError(pages *core.Pages, title string, err error) {
	errModal := core.NewModal()
	errModal.SetText(fmt.Sprintf("%s\n\n%v", title, err))
	errModal.AddButtons([]string{"OK"})
	errModal.SetDoneFunc(func(_ int, _ string) {
		pages.RemovePage("ErrorModal")
	})

	pages.AddPage("ErrorModal", errModal, true, true)
}
