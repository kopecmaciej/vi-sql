package modal

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/rs/zerolog/log"
)

const (
	ErrorModalId = "Error"
)

func NewError(message string, err error) *tview.Modal {
	taggedMessage := "[White::b] " + message + " [::]"

	if err != nil {
		errMsg := err.Error()
		if errMsg != "" {
			if len(errMsg) > 240 {
				errMsg = errMsg[:240] + " ..."
			}
			taggedMessage += "\n" + errMsg
		}
	}

	errModal := tview.NewModal()
	errModal.SetTitle(" Error ")
	errModal.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	errModal.SetTextColor(tcell.ColorRed)
	errModal.SetText(taggedMessage)

	return errModal
}

// ShowError shows a modal with an error message and logs it.
func ShowError(page *core.Pages, message string, err error) {
	log.Error().Err(err).Msg(message)
	errModal := NewError(message, err)
	errModal.AddButtons([]string{"Ok"})

	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Ok" {
			page.RemovePage(ErrorModalId)
		}
	})
	page.AddPage(ErrorModalId, errModal, true, true)
}

// ShowErrorAndSetFocus shows an error modal, logs it, and restores focus on dismiss.
func ShowErrorAndSetFocus(page *core.Pages, message string, err error, setFocus func()) {
	log.Error().Err(err).Msg(message)
	errModal := NewError(message, err)
	errModal.AddButtons([]string{"Ok"})
	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Ok" {
			page.RemovePage(ErrorModalId)
			setFocus()
		}
	})
	page.AddPage(ErrorModalId, errModal, true, true)
}

// ShowErrorWithRetry shows an error modal with Fix and Cancel buttons.
// fixFunc is called when the user chooses Fix, allowing them to correct the mistake.
func ShowErrorWithRetry(page *core.Pages, message string, err error, fixFunc func()) {
	log.Error().Err(err).Msg(message)
	errModal := NewError(message, err)
	errModal.AddButtons([]string{"Fix", "Cancel"})
	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		page.RemovePage(ErrorModalId)
		if buttonLabel == "Fix" {
			fixFunc()
		}
	})
	page.AddPage(ErrorModalId, errModal, true, true)
}
