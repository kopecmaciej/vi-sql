package modal

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

// toColumnHints converts database.ColumnInfo slice to util.ColumnHint slice.
func toColumnHints(cols []database.ColumnInfo) []util.ColumnHint {
	hints := make([]util.ColumnHint, len(cols))
	for i, c := range cols {
		hints[i] = util.ColumnHint{
			Name:       c.Name,
			IsNullable: c.IsNullable,
			HasDefault: c.Default != nil,
			IsPK:       c.IsPK,
		}
	}
	return hints
}

const ImportModalId = "ImportModal"

// Form item indices.
const (
	importFormTable     = 0
	importFormPath      = 1
	importFormHasHeader = 2
)

// ImportModal is a CSV import dialog.
// table is a "schema.table" string — pre-filled when triggered from the schema
// tree, otherwise typed manually with autocomplete.
type ImportModal struct {
	*core.BaseElement
	*core.Flex

	form         *core.Form
	previewTable *core.Table
	warningsView *core.TextView

	schemas []database.Schema
	columns []database.ColumnInfo // populated on Preview
}

func NewImportModal() *ImportModal {
	m := &ImportModal{
		BaseElement:  core.NewBaseElement(),
		Flex:         core.NewFlex(),
		form:         core.NewForm(),
		previewTable: core.NewTable(),
		warningsView: core.NewTextView(),
	}
	m.SetIdentifier(ImportModalId)
	m.SetAfterInitFunc(m.init)
	return m
}

func (m *ImportModal) init() error {
	m.setLayout()
	m.setStyle()
	m.handleEvents()
	return nil
}

func (m *ImportModal) handleEvents() {
	go m.HandleEvents(ImportModalId, func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			m.setStyle()
		}
	})
}

// SetSchemas provides the schema+table list used for autocomplete.
func (m *ImportModal) SetSchemas(schemas []database.Schema) {
	m.schemas = schemas
}

func (m *ImportModal) setStyle() {
	s := m.App.GetStyles()
	m.Flex.SetStyle(s)
	m.previewTable.SetStyle(s)
	m.warningsView.SetStyle(s)
	if m.form != nil {
		m.form.SetStyle(s)
		m.form.SetLabelColor(s.Global.SecondaryTextColor.Color())
		m.form.SetFieldBackgroundColor(s.Global.ContrastBackgroundColor.Color())
		m.form.SetFieldTextColor(s.Global.TextColor.Color())
	}
}

func (m *ImportModal) setLayout() {
	m.Flex.SetDirection(tview.FlexRow)
	m.Flex.SetBorder(true)
	m.Flex.SetTitle(" Import CSV ")
	m.Flex.SetBorderPadding(0, 0, 2, 2)

	m.previewTable.SetBorder(true)
	m.previewTable.SetTitle(" Preview (first 5 rows) ")
	m.previewTable.SetFixed(1, 0)
	m.previewTable.SetSelectable(false, false)

	m.warningsView.SetDynamicColors(true)
	m.warningsView.SetWrap(true)
}

func (m *ImportModal) rebuildContent() {
	m.Flex.Clear()
	m.Flex.AddItem(m.form, 9, 0, true)
	m.Flex.AddItem(m.previewTable, 0, 1, false)
	m.Flex.AddItem(m.warningsView, 2, 0, false)
}

// Render opens the import dialog. schema and table pre-fill the Table field
// as "schema.table"; pass empty strings when not triggered from the schema tree.
func (m *ImportModal) Render(schema, table string) {
	m.columns = nil

	prefill := ""
	if schema != "" && table != "" {
		prefill = schema + "." + table
	}

	m.buildForm(prefill)
	m.previewTable.Clear()
	m.warningsView.SetText("")
	m.rebuildContent()
	m.setStyle()

	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(m.Flex, 0, 7, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
		AddItem(nil, 0, 1, false)

	m.App.Pages.AddPage(ImportModalId, wrapper, true, true)
	m.App.SetFocusOnly(m.form)
}

func (m *ImportModal) buildForm(tableValue string) {
	m.form.Clear(true)

	m.form.AddInputFieldWithAutocomplete(
		"Table:     ",
		tableValue,
		0,
		m.autocompleteTable,
		nil,
		nil,
	)
	m.form.AddInputField("Path:      ", "", 0, nil, nil)
	m.form.AddCheckbox("Has Header ", true, nil)

	m.form.AddButton(" Preview ", func() { m.loadPreview() })
	m.form.AddButton(" Import  ", func() { m.doImport() })
	m.form.AddButton(" Cancel  ", func() { m.App.Pages.RemovePage(ImportModalId) })
	m.form.SetCancelFunc(func() { m.App.Pages.RemovePage(ImportModalId) })
	m.form.ApplyFormNavKeys(m.App.GetKeys())
	m.applyAutocompleteKeys()
}

// applyAutocompleteKeys wires the configured autocomplete navigation keys onto
// the Table input field so Ctrl+p/Ctrl+n/Ctrl+y (or whatever the user has set)
// move through the autocomplete list in addition to the default Up/Down/Enter.
func (m *ImportModal) applyAutocompleteKeys() {
	field, ok := m.form.GetFormItem(importFormTable).(*tview.InputField)
	if !ok {
		return
	}
	k := m.App.GetKeys()
	existing := field.GetInputCapture()
	field.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.AutocompleteUp, event):
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case k.Match(k.Navigation.AutocompleteDown, event):
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case k.Match(k.Navigation.AutocompleteAccept, event):
			return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		}
		if existing != nil {
			return existing(event)
		}
		return event
	}))
}

func (m *ImportModal) autocompleteTable(text string) []tview.AutocompleteItem {
	lower := strings.ToLower(text)
	var entries []tview.AutocompleteItem
	for _, s := range m.schemas {
		for _, t := range s.Tables {
			full := s.Schema + "." + t
			if strings.Contains(strings.ToLower(full), lower) {
				entries = append(entries, tview.AutocompleteItem{Main: full})
			}
		}
	}
	return entries
}

func (m *ImportModal) loadPreview() {
	schema, table, ok := m.parsedTable()
	if !ok {
		m.warningsView.SetText("[red]Enter table as schema.table")
		return
	}
	path := m.resolvedPath()
	if path == "" {
		m.warningsView.SetText("[red]Path is required")
		return
	}

	ctx := context.Background()
	var err error
	m.columns, err = m.Driver.GetTableColumns(ctx, schema, table)
	if err != nil {
		ShowError(m.App.Pages, "Import: cannot fetch table columns", err)
		return
	}

	hasHeader := m.hasHeaderChecked()
	headers, rows, err := util.ParseCSVPreview(path, hasHeader, 5)
	if err != nil {
		ShowError(m.App.Pages, "Import: cannot read CSV", err)
		return
	}

	m.previewTable.Clear()
	s := m.App.GetStyles()

	colCount := 0
	if len(rows) > 0 {
		colCount = len(rows[0])
	} else if len(headers) > 0 {
		colCount = len(headers)
	}

	for c := 0; c < colCount; c++ {
		label := fmt.Sprintf("Col %d", c+1)
		if hasHeader && c < len(headers) {
			label = headers[c]
		}
		m.previewTable.SetCell(0, c, tview.NewTableCell(" "+label+" ").
			SetSelectable(false).
			SetTextColor(s.Global.SecondaryTextColor.Color()).
			SetBackgroundColor(s.Global.ContrastBackgroundColor.Color()))
	}
	for r, row := range rows {
		for c, val := range row {
			m.previewTable.SetCell(r+1, c, tview.NewTableCell(" "+val+" ").
				SetTextColor(s.Global.TextColor.Color()))
		}
	}

	csvHeaders := m.csvHeadersFor(headers, hasHeader, colCount)
	mapping, mapErr := util.BuildColumnMapping(csvHeaders, toColumnHints(m.columns))
	if mapErr != nil {
		m.warningsView.SetText(fmt.Sprintf("[red]ERROR: %s", mapErr.Error()))
		return
	}
	if len(mapping.Warnings) > 0 {
		m.warningsView.SetText("[yellow]" + strings.Join(mapping.Warnings, "\n"))
	} else {
		m.warningsView.SetText("[green]Column mapping OK")
	}
}

func (m *ImportModal) doImport() {
	schema, table, ok := m.parsedTable()
	if !ok {
		ShowError(m.App.Pages, "Import: enter table as schema.table", nil)
		return
	}
	path := m.resolvedPath()
	if path == "" {
		ShowError(m.App.Pages, "Import: path is required", nil)
		return
	}
	hasHeader := m.hasHeaderChecked()

	// Fetch columns if Preview was not clicked (or table changed since).
	if m.columns == nil {
		ctx := context.Background()
		var err error
		m.columns, err = m.Driver.GetTableColumns(ctx, schema, table)
		if err != nil {
			ShowError(m.App.Pages, "Import: cannot fetch table columns", err)
			return
		}
	}

	headers, _, err := util.ParseCSVPreview(path, hasHeader, 0)
	if err != nil {
		ShowError(m.App.Pages, "Import: cannot read CSV", err)
		return
	}

	colCount := len(headers)
	if !hasHeader && colCount == 0 {
		colCount = len(m.columns)
	}
	csvHeaders := m.csvHeadersFor(headers, hasHeader, colCount)

	mapping, mapErr := util.BuildColumnMapping(csvHeaders, toColumnHints(m.columns))
	if mapErr != nil {
		ShowError(m.App.Pages, "Import: column mapping error", mapErr)
		return
	}

	opts := util.ImportOptions{HasHeader: hasHeader, BatchSize: 50}
	m.App.Pages.RemovePage(ImportModalId)
	go m.performImport(path, opts, mapping, schema, table)
}

func (m *ImportModal) performImport(
	path string,
	opts util.ImportOptions,
	mapping util.MappingResult,
	schema, table string,
) {
	ctx := context.Background()

	insertFn := func(row map[string]any) error {
		_, err := m.Driver.InsertRow(ctx, schema, table, row)
		return err
	}

	imported, errs := util.ImportRows(ctx, path, opts, mapping, nil, insertFn)

	m.App.QueueUpdateDraw(func() {
		if len(errs) == 0 {
			ShowError(m.App.Pages, fmt.Sprintf("Import complete: %d rows inserted.", imported), nil)
			return
		}
		limit := 10
		if len(errs) < limit {
			limit = len(errs)
		}
		msgs := make([]string, limit)
		for i := 0; i < limit; i++ {
			msgs[i] = errs[i].Error()
		}
		summary := fmt.Sprintf("%d rows inserted, %d errors:\n%s", imported, len(errs), strings.Join(msgs, "\n"))
		if len(errs) > 10 {
			summary += fmt.Sprintf("\n… and %d more", len(errs)-10)
		}
		ShowError(m.App.Pages, summary, nil)
	})
}

func (m *ImportModal) csvHeadersFor(headers []string, hasHeader bool, colCount int) []string {
	if hasHeader {
		return headers
	}
	n := colCount
	if len(m.columns) < n {
		n = len(m.columns)
	}
	synth := make([]string, n)
	for i := 0; i < n; i++ {
		synth[i] = m.columns[i].Name
	}
	return synth
}

// parsedTable splits the Table field value ("schema.table") into parts.
func (m *ImportModal) parsedTable() (schema, table string, ok bool) {
	raw := strings.TrimSpace(m.form.GetFormItem(importFormTable).(*tview.InputField).GetText())
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (m *ImportModal) resolvedPath() string {
	raw := strings.TrimSpace(m.form.GetFormItem(importFormPath).(*tview.InputField).GetText())
	if raw == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil {
		raw = strings.Replace(raw, "~", home, 1)
	}
	return raw
}

func (m *ImportModal) hasHeaderChecked() bool {
	cb, ok := m.form.GetFormItem(importFormHasHeader).(*tview.Checkbox)
	return ok && cb.IsChecked()
}
