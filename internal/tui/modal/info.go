package modal

import (
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const InfoModalId = "Info"

// ShowInfo shows a simple informational modal with an Ok button.
func ShowInfo(page *core.Pages, message string) {
	m := tview.NewModal()
	m.SetTitle(" Info ")
	m.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	m.SetText(message)
	m.AddButtons([]string{"Ok"})
	m.SetDoneFunc(func(_ int, _ string) {
		page.RemoveModalPage(InfoModalId)
	})
	page.AddModalPage(InfoModalId, m, true, true)
}
