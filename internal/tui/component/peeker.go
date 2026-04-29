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
	p.SetBorder(true)
	p.SetTitle(" Row Details ")
	p.SetTitleAlign(tview.AlignLeft)

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
	p.ViewModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Navigation.GoTop, event.Name()):
			p.ViewModal.MoveToTop()
			return nil
		case k.Contains(k.Navigation.GoBottom, event.Name()):
			p.ViewModal.MoveToBottom()
			return nil
		case k.Contains(k.Peeker.CopyHighlight, event.Name()):
			p.ViewModal.CopySelectedLine(util.Copy, "full")
			return nil
		case k.Contains(k.Common.Copy, event.Name()):
			p.ViewModal.CopySelectedLine(util.Copy, "value")
			return nil
		case k.Contains(k.Peeker.ExpandRow, event.Name()):
			p.ViewModal.ToggleExpand()
			return nil
		case k.Contains(k.Peeker.OpenValueViewer, event.Name()):
			p.openValueViewer()
			return nil
		case k.Contains(k.Peeker.ToggleFullScreen, event.Name()):
			p.ViewModal.SetFullScreen(!p.ViewModal.IsFullScreen())
			p.ViewModal.MoveToTop()
			return nil
		case k.Contains(k.Common.Close, event.Name()):
			p.App.Pages.RemovePage(p.GetIdentifier())
			return nil
		}
		return event
	})
}

func (p *Peeker) SetDoneFunc(doneFunc func()) {
	p.doneFunc = doneFunc
}

const valueViewerPageId = "ValueViewer"

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

	styles := p.App.GetStyles()
	viewer := primitives.NewValueViewer()
	viewer.SetBorder(true)
	viewer.SetBackgroundColor(styles.Global.BackgroundColor.Color())
	viewer.SetBorderColor(styles.Global.BorderColor.Color())
	viewer.SetTitleColor(styles.Global.TitleColor.Color())
	viewer.SetTextColor(styles.Global.TextColor.Color())
	viewer.SetContent(title, content)
	viewer.SetDoneFunc(func() {
		p.App.Pages.RemovePage(valueViewerPageId)
	})

	p.App.Pages.AddPage(valueViewerPageId, viewer, true, true)
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
		lines = append(lines, primitives.RowLine{
			Key:         col.Name,
			Type:        col.DataType,
			Value:       val,
			IsPK:        col.IsPK,
			PrettyValue: prettyFormatValue(val, col.DataType),
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
