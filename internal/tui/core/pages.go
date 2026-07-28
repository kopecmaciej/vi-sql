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

// AddPage is a plain pass-through to tview.Pages.AddPage. It does NOT touch
// the focus stack. Use it for top-level pages (Main, Connection, Options).
// Transient overlays that should restore focus on dismiss must use
// AddModalPage / RemoveModalPage instead.
func (p *Pages) AddPage(view tview.Identifier, page tview.Primitive, resize, visible bool) *tview.Pages {
	return p.Pages.AddPage(string(view), page, resize, visible)
}

// AddModalPage snapshots the current focus and adds the page. The captured
// focus is restored when the page is dismissed via RemoveModalPage. The caller
// is responsible for SetFocusOnly-ing the focused primitive (or a sub-widget
// of it) after this call.
func (p *Pages) AddModalPage(view tview.Identifier, page tview.Primitive, resize, visible bool) *tview.Pages {
	p.app.SnapshotFocus()
	return p.Pages.AddPage(string(view), page, resize, visible)
}

// RemovePage is a plain pass-through. Use it to dismiss pages added via
// AddPage. Modal pages added with AddModalPage must be dismissed with
// RemoveModalPage so the stack stays balanced.
func (p *Pages) RemovePage(view tview.Identifier) *tview.Pages {
	return p.Pages.RemovePage(string(view))
}

// RemoveModalPage removes a page added via AddModalPage and restores the
// focus captured at open time.
func (p *Pages) RemoveModalPage(view tview.Identifier) *tview.Pages {
	p.Pages.RemovePage(string(view))
	p.app.RestoreFocus()
	return p.Pages
}

// HasPage wraps tview.Pages.HasPage.
func (p *Pages) HasPage(view tview.Identifier) bool {
	return p.Pages.HasPage(string(view))
}