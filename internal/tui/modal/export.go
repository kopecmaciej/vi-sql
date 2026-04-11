package modal

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const ExportModalId = "ExportModal"

var exportFormats = []struct {
	format      util.ExportFormat
	description string
}{
	{util.ExportCSV, "comma-separated, importable everywhere"},
	{util.ExportJSON, "array of objects"},
	{util.ExportSQLInsert, "INSERT statements"},
}

// ExportModal handles the two-step export flow: format selection → file path input.
type ExportModal struct {
	*core.BaseElement

	style *config.OthersStyle
}

func NewExportModal() *ExportModal {
	e := &ExportModal{
		BaseElement: core.NewBaseElement(),
	}
	e.SetAfterInitFunc(e.init)
	return e
}

func (e *ExportModal) init() error {
	e.setStyle()
	e.handleEvents()
	return nil
}

func (e *ExportModal) setStyle() {
	e.style = &e.App.GetStyles().Others
}

func (e *ExportModal) handleEvents() {
	go e.HandleEvents(ExportModalId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			e.setStyle()
		}
	})
}


// Show starts the export flow for the given query and table context.
// query is the full SQL to execute (no LIMIT/OFFSET); schema and table are
// used for SQL INSERT target and for the default filename suggestion.
func (e *ExportModal) Show(ctx context.Context, query, schema, table string) {
	e.showFormatStep(ctx, query, schema, table)
}

func (e *ExportModal) showFormatStep(ctx context.Context, query, schema, table string) {
	s := e.App.GetStyles()
	bg := s.Global.BackgroundColor.Color()

	fmtModal := primitives.NewListModal()
	fmtModal.SetBorder(true)
	fmtModal.SetTitle(" Export Format ")
	fmtModal.SetBorderColor(s.Global.BorderColor.Color())
	fmtModal.SetBackgroundColor(bg)
	fmtModal.ShowSecondaryText(true)
	fmtModal.SetBorderPadding(0, 0, 1, 1)

	mainStyle := tcell.StyleDefault.
		Foreground(e.App.GetStyles().Global.SecondaryTextColor.Color()).
		Background(bg)
	fmtModal.SetMainTextStyle(mainStyle)

	secondaryStyle := tcell.StyleDefault.
		Foreground(e.App.GetStyles().Global.TitleColor.Color()).
		Background(bg)
	fmtModal.SetSecondaryTextStyle(secondaryStyle)

	selectedStyle := tcell.StyleDefault.
		Foreground(bg).
		Background(s.Global.FocusColor.Color())
	fmtModal.SetSelectedStyle(selectedStyle)

	for _, f := range exportFormats {
		fmtModal.AddItem(string(f.format), f.description, 0, nil)
	}

	fmtModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.App.Pages.RemovePage(ExportModalId)
			return nil
		case tcell.KeyEnter:
			chosen := util.ExportFormat(fmtModal.GetText())
			e.App.Pages.RemovePage(ExportModalId)
			e.showPathStep(ctx, chosen, query, schema, table)
			return nil
		}
		return event
	})

	e.App.Pages.AddPage(ExportModalId, fmtModal, true, true)
}

func (e *ExportModal) showPathStep(ctx context.Context, format util.ExportFormat, query, schema, table string) {
	s := e.App.GetStyles()
	bg := s.Global.BackgroundColor.Color()

	pathModal := primitives.NewInputModal()
	pathModal.SetBorder(true)
	pathModal.SetTitle(fmt.Sprintf(" Export as %s ", format))
	pathModal.SetBackgroundColor(bg)
	pathModal.SetBorderColor(s.Global.BorderColor.Color())
	pathModal.SetFieldBackgroundColor(s.Global.ContrastBackgroundColor.Color())
	pathModal.SetFieldTextColor(s.Global.TextColor.Color())
	pathModal.SetLabelColor(e.App.GetStyles().Global.SecondaryTextColor.Color())
	pathModal.SetInputLabel("File: ")
	pathModal.SetText(e.defaultFilename(format, table))

	pathModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			e.App.Pages.RemovePage(ExportModalId)
			return nil
		case tcell.KeyEnter:
			path := strings.TrimSpace(pathModal.GetText())
			e.App.Pages.RemovePage(ExportModalId)
			if path != "" {
				if home, err := os.UserHomeDir(); err == nil {
					path = strings.Replace(path, "~", home, 1)
				}
				go e.performExport(ctx, format, path, query, schema, table)
			}
			return nil
		}
		return event
	})

	e.App.Pages.AddPage(ExportModalId, pathModal, true, true)
}

func (e *ExportModal) performExport(ctx context.Context, format util.ExportFormat, path, query, schema, table string) {
	rows, cols, err := e.Driver.ExecuteQuery(ctx, query)
	if err != nil {
		e.App.QueueUpdateDraw(func() {
			ShowError(e.App.Pages, "Export: query failed", err)
		})
		return
	}

	columns := make([]string, len(cols))
	for i, col := range cols {
		columns[i] = col.Name
	}

	f, err := os.Create(path)
	if err != nil {
		e.App.QueueUpdateDraw(func() {
			ShowError(e.App.Pages, "Export: cannot create file", err)
		})
		return
	}
	defer f.Close()

	if err := util.ExportRows(f, format, columns, rows, schema, table); err != nil {
		e.App.QueueUpdateDraw(func() {
			ShowError(e.App.Pages, "Export: write failed", err)
		})
		return
	}

	e.App.QueueUpdateDraw(func() {
		ShowError(e.App.Pages, fmt.Sprintf("Exported %d rows to %s", len(rows), path), nil)
	})
}

func (e *ExportModal) defaultFilename(format util.ExportFormat, table string) string {
	name := table
	if name == "" {
		name = "query"
	}
	ext := map[util.ExportFormat]string{
		util.ExportCSV:       ".csv",
		util.ExportJSON:      ".json",
		util.ExportSQLInsert: ".sql",
	}[format]
	return name + ext
}
