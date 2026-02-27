package component

import (
	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
	"github.com/rs/zerolog/log"
)

const (
	PeekerId = "Peeker"
)

// Peeker displays a single database row in a vertical key/type/value format.
type Peeker struct {
	*core.BaseElement
	*core.ViewModal

	doneFunc func()
}

func NewPeeker() *Peeker {
	p := &Peeker{
		BaseElement: core.NewBaseElement(),
		ViewModal:   core.NewViewModal(),
	}

	p.SetIdentifier(PeekerId)
	p.SetAfterInitFunc(p.init)

	return p
}

func (p *Peeker) init() error {
	p.setStyle()
	p.setLayout()
	p.setKeybindings()
	p.handleEvents()

	return nil
}

func (p *Peeker) handleEvents() {
	go p.HandleEvents(PeekerId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			p.setStyle()
		case manager.HeaderHeightChanged:
			if h, ok := event.Message.Data.(int); ok {
				p.ViewModal.SetTopOffset(h)
			}
		}
	})
}

func (p *Peeker) setLayout() {
	p.SetBorder(true)
	p.SetTitle(" Row Details ")
	p.SetTitleAlign(tview.AlignLeft)

	p.ViewModal.AddButtons([]string{"Close"})
}

func (p *Peeker) setStyle() {
	style := &p.App.GetStyles().RowPeeker
	p.ViewModal.SetStyle(p.App.GetStyles())
	p.SetHighlightColor(style.HighlightColor.Color())
	p.SetDocumentColors(
		style.KeyColor.Color(),
		style.ValueColor.Color(),
		style.BracketColor.Color(),
	)
}

func (p *Peeker) setKeybindings() {
	k := p.App.GetKeys()
	p.ViewModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Peeker.MoveToTop, event.Name()):
			p.ViewModal.MoveToTop()
			return nil
		case k.Contains(k.Peeker.MoveToBottom, event.Name()):
			p.ViewModal.MoveToBottom()
			return nil
		case k.Contains(k.Peeker.CopyHighlight, event.Name()):
			if err := p.ViewModal.CopySelectedLine(clipboard.WriteAll, "full"); err != nil {
				log.Error().Err(err).Msg("Error copying full line")
				modal.ShowError(p.App.Pages, "Error copying full line", err)
			}
			return nil
		case k.Contains(k.Peeker.CopyValue, event.Name()):
			if err := p.ViewModal.CopySelectedLine(clipboard.WriteAll, "value"); err != nil {
				log.Error().Err(err).Msg("Error copying value")
				modal.ShowError(p.App.Pages, "Error copying value", err)
			}
			return nil
		case k.Contains(k.Peeker.ExpandRow, event.Name()):
			p.ViewModal.ToggleExpand()
			return nil
		case k.Contains(k.Peeker.ToggleFullScreen, event.Name()):
			p.ViewModal.SetFullScreen(!p.ViewModal.IsFullScreen())
			p.ViewModal.MoveToTop()
			return nil
		case k.Contains(k.Peeker.Exit, event.Name()):
			p.App.Pages.RemovePage(p.GetIdentifier())
			return nil
		}
		return event
	})
}

func (p *Peeker) SetDoneFunc(doneFunc func()) {
	p.doneFunc = doneFunc
}

// Render converts a database Row and its column metadata into RowLines
// and displays the modal.
func (p *Peeker) Render(row database.Row, columns []database.ColumnInfo) {
	p.ViewModal.MoveToTop()

	lines := make([]primitives.RowLine, 0, len(columns))
	for _, col := range columns {
		val := database.StringifyValue(row[col.Name])
		lines = append(lines, primitives.RowLine{
			Key:   col.Name,
			Type:  col.DataType,
			Value: val,
			IsPK:  col.IsPK,
		})
	}

	p.ViewModal.SetRows(lines)

	p.App.Pages.AddPage(p.GetIdentifier(), p.ViewModal, true, true)
	p.ViewModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Close" || buttonLabel == "" {
			p.App.Pages.RemovePage(p.GetIdentifier())
		}
	})
}
