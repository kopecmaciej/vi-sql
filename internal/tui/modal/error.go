package modal

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

const (
	ErrorModalId = "Error"
)

func newError(message string, err error) (*tview.Modal, string) {
	plain := message
	taggedMessage := "[White::b] " + message + " [::]"

	if err != nil {
		errMsg := err.Error()
		if errMsg != "" {
			if len(errMsg) > 240 {
				errMsg = errMsg[:240] + " ..."
			}
			plain += "\n" + errMsg
			taggedMessage += "\n" + errMsg
		}
	}

	errModal := tview.NewModal()
	errModal.SetTitle(" Error ")
	errModal.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	errModal.SetTextColor(tcell.ColorRed)
	errModal.SetText(taggedMessage)

	return errModal, plain
}

// ShowError shows a modal with an error message and logs it.
func ShowError(page *core.Pages, message string, err error) {
	log.Error().Err(err).Msg(message)
	errModal, plain := newError(message, err)
	errModal.AddButtons([]string{"Close", "Copy"})

	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		page.RemoveModalPage(ErrorModalId)
		if buttonLabel == "Copy" {
			util.Copy(plain)
		}
	})
	page.AddModalPage(ErrorModalId, errModal, true, true)
}

// ShowErrorAndSetFocus shows an error modal, logs it, and restores focus on dismiss.
func ShowErrorAndSetFocus(page *core.Pages, message string, err error, setFocus func()) {
	log.Error().Err(err).Msg(message)
	errModal, plain := newError(message, err)
	errModal.AddButtons([]string{"Close", "Copy"})
	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		page.RemoveModalPage(ErrorModalId)
		if buttonLabel == "Copy" {
			util.Copy(plain)
		}
		setFocus()
	})
	page.AddModalPage(ErrorModalId, errModal, true, true)
}

// ShowErrorWithDone shows an error modal and calls onDone after the user dismisses it.
// Esc dismisses without calling onDone.
func ShowErrorWithDone(page *core.Pages, message string, err error, onDone func()) {
	log.Error().Err(err).Msg(message)
	errModal, plain := newError(message, err)
	errModal.AddButtons([]string{"Close", "Copy"})
	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		page.RemoveModalPage(ErrorModalId)
		if buttonLabel == "Copy" {
			util.Copy(plain)
		}
		if buttonIndex != -1 && onDone != nil {
			onDone()
		}
	})
	page.AddModalPage(ErrorModalId, errModal, true, true)
}

// ShowErrorWithRetry shows an error modal with Fix and Cancel buttons.
// fixFunc is called when the user chooses Fix, allowing them to correct the mistake.
func ShowErrorWithRetry(page *core.Pages, message string, err error, fixFunc func()) {
	log.Error().Err(err).Msg(message)
	errModal, plain := newError(message, err)
	errModal.AddButtons([]string{"Fix", "Copy", "Cancel"})
	errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		page.RemoveModalPage(ErrorModalId)
		switch buttonLabel {
		case "Copy":
			util.Copy(plain)
		case "Fix":
			fixFunc()
		}
	})
	page.AddModalPage(ErrorModalId, errModal, true, true)
}
