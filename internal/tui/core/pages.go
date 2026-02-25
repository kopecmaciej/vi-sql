package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
)

type Pages struct {
	*tview.Pages

	manager *manager.ElementManager
	app     *App
}

func NewPages(manager *manager.ElementManager, app *App) *Pages {
	return &Pages{
		Pages:   tview.NewPages(),
		manager: manager,
		app:     app,
	}
}

func (p *Pages) SetStyle(style *config.Styles) {
	p.Pages.SetBackgroundColor(style.Global.BackgroundColor.Color())
	p.Pages.SetBorderColor(style.Global.BorderColor.Color())
	p.Pages.SetTitleColor(style.Global.TitleColor.Color())
	p.Pages.SetFocusStyle(tcell.StyleDefault.
		Foreground(style.Global.FocusColor.Color()).
		Background(style.Global.BackgroundColor.Color()))
}

// AddPage wraps tview.Pages.AddPage with focus tracking.
func (p *Pages) AddPage(view tview.Identifier, page tview.Primitive, resize, visible bool) *tview.Pages {
	p.app.SetPreviousFocus()
	p.Pages.AddPage(string(view), page, resize, visible)
	if visible && page.HasFocus() {
		p.app.FocusChanged(page)
	}
	return p.Pages
}

// RemovePage wraps tview.Pages.RemovePage with focus restoration.
func (p *Pages) RemovePage(view tview.Identifier) *tview.Pages {
	p.Pages.RemovePage(string(view))
	p.app.GiveBackFocus()
	return p.Pages
}

// HasPage wraps tview.Pages.HasPage.
func (p *Pages) HasPage(view tview.Identifier) bool {
	return p.Pages.HasPage(string(view))
}
