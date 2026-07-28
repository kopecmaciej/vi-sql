package component

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	PeekerId = "Peeker"
)

var peekerCounter int32

// Peeker displays a single database row in a vertical key/type/value format.
type Peeker struct {
	*core.BaseElement
	*core.ViewModal

	doneFunc func()
}

func NewPeeker() *Peeker {
	n := atomic.AddInt32(&peekerCounter, 1)
	id := tview.Identifier(fmt.Sprintf("%s-%d", PeekerId, n))
	p := &Peeker{
		BaseElement: core.NewBaseElement(),
		ViewModal:   core.NewViewModal(),
	}

	p.SetIdentifier(id)
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
	go p.HandleEvents(p.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			p.setStyle()
		case manager.FooterHeightChanged:
			if h, ok := event.Message.Data.(int); ok {
				p.ViewModal.SetTopOffset(h)
			}
		}
	})
}

func (p *Peeker) setLayout() {
	p.ViewModal.SetFrameTitle(" Row Details ")

	p.ViewModal.SetCentered(true)
	p.ViewModal.AddButtons([]string{"Close"})
}

func (p *Peeker) setStyle() {
	styles := p.App.GetStyles()
	p.ViewModal.SetStyle(styles)
	p.SetHighlightColor(styles.Others.PeekerHighlightColor.Color())
	p.SetDocumentColors(
		styles.Global.TitleColor.Color(),
		styles.Global.TextColor.Color(),
		styles.Global.SecondaryTextColor.Color(),
	)
}

func (p *Peeker) setKeybindings() {
	k := p.App.GetKeys()
	p.ViewModal.SetKeys(k)
	p.ViewModal.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.GoTop, event):
			p.ViewModal.MoveToTop()
			return nil
		case k.Match(k.Navigation.GoBottom, event):
			p.ViewModal.MoveToBottom()
			return nil
		case k.Match(k.Peeker.CopyHighlight, event):
			p.ViewModal.CopySelectedLine(util.Copy, "full")
			return nil
		case k.Match(k.Common.Copy, event):
			p.ViewModal.CopySelectedLine(util.Copy, "value")
			return nil
		case k.Match(k.Peeker.ExpandRow, event):
			p.ViewModal.ToggleExpand()
			return nil
		case k.Match(k.Peeker.OpenValueViewer, event):
			p.openValueViewer()
			return nil
		case k.Match(k.Peeker.ToggleFullScreen, event):
			p.ViewModal.SetFullScreen(!p.ViewModal.IsFullScreen())
			p.ViewModal.MoveToTop()
			return nil
		case k.Match(k.Common.Close, event):
			p.App.Pages.RemoveModalPage(p.GetIdentifier())
			return nil
		}
		return event
	}))
}

func (p *Peeker) SetDoneFunc(doneFunc func()) {
	p.doneFunc = doneFunc
}

const valueViewerPageSuffix = "-viewer"

// openValueViewer opens a full-screen scrollable viewer for the currently
// selected row's value. If the row has a PrettyValue (JSON/XML) it is shown
// formatted; otherwise the raw value is used.
func (p *Peeker) openValueViewer() {
	rl, ok := p.ViewModal.SelectedRow()
	if !ok {
		return
	}

	content := rl.PrettyValue
	if content == "" {
		content = rl.Value
	}
	if content == "" {
		return
	}

	title := rl.Key + " (" + rl.Type + ")"
	viewerPageId := tview.Identifier(string(p.GetIdentifier()) + valueViewerPageSuffix)

	styles := p.App.GetStyles()
	k := p.App.GetKeys()
	viewer := core.NewTextView()
	viewer.SetScrollable(true)
	viewer.SetWrap(true)
	viewer.SetBorderPadding(0, 0, 1, 1)
	viewer.SetBorder(true)
	viewer.SetTitle(" " + title + " ")
	viewer.SetTitleAlign(tview.AlignLeft)
	viewer.SetBackgroundColor(styles.Global.BackgroundColor.Color())
	viewer.SetBorderColor(styles.Global.BorderColor.Color())
	viewer.SetTitleColor(styles.Global.TitleColor.Color())
	viewer.SetTextColor(styles.Global.TextColor.Color())
	viewer.SetText(content)
	viewer.ScrollToBeginning()
	viewer.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := viewer.GetScrollOffset()
		switch {
		case k.Match(k.Common.Close, event):
			p.App.Pages.RemoveModalPage(viewerPageId)
			return nil
		case k.Match(k.Navigation.GoTop, event):
			viewer.ScrollToBeginning()
			return nil
		case k.Match(k.Navigation.GoBottom, event):
			viewer.ScrollToEnd()
			return nil
		case k.Match(k.Navigation.MoveUp, event):
			if row > 0 {
				viewer.ScrollTo(row-1, col)
			}
			return nil
		case k.Match(k.Navigation.MoveDown, event):
			viewer.ScrollTo(row+1, col)
			return nil
		}
		return event
	}))

	p.App.Pages.AddModalPage(viewerPageId, viewer, true, true)
	p.App.SetFocusOnly(viewer)
}

// prettyFormatValue returns a human-readable multi-line representation of val
// for types that benefit from structured formatting. Returns "" when no
// special formatting applies (caller falls back to plain word-wrap).
func prettyFormatValue(val, dataType string) string {
	dt := strings.ToLower(dataType)
	switch dt {
	case "json", "jsonb":
		var buf bytes.Buffer
		if err := json.Indent(&buf, []byte(val), "", "  "); err == nil {
			return buf.String()
		}
	case "xml":
		var buf bytes.Buffer
		d := xml.NewDecoder(strings.NewReader(val))
		e := xml.NewEncoder(&buf)
		e.Indent("", "  ")
		for {
			tok, err := d.Token()
			if err != nil {
				break
			}
			if err := e.EncodeToken(tok); err != nil {
				return ""
			}
		}
		if err := e.Flush(); err != nil {
			return ""
		}
		if result := buf.String(); result != "" {
			return result
		}
	}
	return ""
}

// Render converts a database Row and its column metadata into RowLines
// and displays the modal.
func (p *Peeker) Render(row database.Row, columns []database.ColumnInfo) {
	p.ViewModal.MoveToTop()

	lines := make([]primitives.RowLine, 0, len(columns))
	for _, col := range columns {
		val := database.StringifyValue(row[col.Name])
		typ := col.DataType
		if col.IsPK {
			typ += " (PK)"
		}
		if col.IsFK {
			typ += " (FK)"
		}
		lines = append(lines, primitives.RowLine{
			Key:         col.Name,
			Type:        typ,
			Value:       val,
			PrettyValue: prettyFormatValue(val, col.DataType),
		})
	}

	p.ViewModal.SetRows(lines)

	p.App.Pages.AddModalPage(p.GetIdentifier(), p.ViewModal, true, true)
	p.App.SetFocusOnly(p.ViewModal)
	p.ViewModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Close" || buttonLabel == "" {
			p.App.Pages.RemoveModalPage(p.GetIdentifier())
		}
	})
}
