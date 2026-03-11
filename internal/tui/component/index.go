package component

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
)

const (
	IndexId            = "Index"
	IndexDeleteModalId = "IndexDeleteModal"
)

type Indexes struct {
	*core.BaseElement
	*core.Flex

	table       *core.Table
	deleteModal *modal.Confirm
	addForm     *core.Form

	schema           string
	tbl              string
	indexes          []database.IndexInfo
	colKeys          []string
	columnCount      int
	isAddFormVisible bool
	isRawMode        bool
}

func NewIndexes() *Indexes {
	idx := &Indexes{
		BaseElement:      core.NewBaseElement(),
		Flex:             core.NewFlex(),
		table:            core.NewTable(),
		deleteModal:      modal.NewConfirm(IndexDeleteModalId),
		addForm:          core.NewForm(),
		isAddFormVisible: false,
	}

	idx.SetIdentifier(IndexId)
	idx.table.SetIdentifier(IndexId)
	idx.SetAfterInitFunc(idx.init)

	return idx
}

func (idx *Indexes) init() error {
	idx.setStyle()
	idx.setLayout()
	idx.setKeybindings()

	if err := idx.deleteModal.Init(idx.App); err != nil {
		return err
	}

	idx.handleEvents()
	return nil
}

func (idx *Indexes) setStyle() {
	styles := idx.App.GetStyles()
	idx.Flex.SetStyle(styles)
	idx.table.SetStyle(styles)
	idx.addForm.SetStyle(styles)
	idx.Flex.SetBorderColor(styles.Others.SeparatorColor.Color())
	idx.table.SetBordersColor(styles.Others.SeparatorColor.Color())
}

func (idx *Indexes) setLayout() {
	idx.Flex.SetBorder(true)
	idx.Flex.SetTitle(" Indexes ")
	idx.Flex.SetTitleAlign(tview.AlignCenter)
	idx.Flex.SetBorderPadding(0, 0, 1, 1)
	idx.Flex.SetDirection(tview.FlexRow)
}

func (idx *Indexes) setKeybindings() {
	k := idx.App.GetKeys()

	idx.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Index.ExitAddIndex, event.Name()):
			if idx.isAddFormVisible {
				idx.closeAddForm()
				return nil
			}
		case k.Contains(k.Index.AddIndex, event.Name()):
			if !idx.isAddFormVisible {
				idx.showAddForm()
				return nil
			}
		case k.Contains(k.Index.DeleteIndex, event.Name()):
			if !idx.isAddFormVisible {
				idx.showDeleteIndexModal(context.Background())
				return nil
			}
		}
		return event
	})
}

func (idx *Indexes) handleEvents() {
	go idx.HandleEvents(IndexId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			idx.setStyle()
			idx.App.QueueUpdateDraw(func() {
				idx.Render()
			})
		}
	})
}

func (idx *Indexes) Render() {
	idx.Flex.Clear()

	if idx.isAddFormVisible {
		idx.Flex.AddItem(idx.addForm, 0, 1, true)
	} else {
		idx.Flex.AddItem(idx.table, 0, 1, true)
	}
}

// HandleTableSelection loads index data for the given schema/table.
func (idx *Indexes) HandleTableSelection(ctx context.Context, schema, table string) {
	idx.schema = schema
	idx.tbl = table
	idx.loadColKeys(ctx)
	idx.loadData(ctx)
}

func (idx *Indexes) loadColKeys(ctx context.Context) {
	keys, err := idx.Driver.GetTableColumnNames(ctx, idx.schema, idx.tbl)
	if err != nil {
		return
	}
	idx.colKeys = keys
}

func (idx *Indexes) loadData(ctx context.Context) {
	if idx.schema == "" || idx.tbl == "" {
		return
	}

	indexes, err := idx.Driver.GetIndexes(ctx, idx.schema, idx.tbl)
	if err != nil {
		modal.ShowError(idx.App.Pages, "Error loading indexes", err)
		return
	}

	idx.indexes = indexes
	idx.renderIndexes(indexes)
}

func (idx *Indexes) renderIndexes(indexes []database.IndexInfo) {
	styles := idx.App.GetStyles()
	idx.table.Clear()
	idx.table.SetFixed(1, 0)
	idx.table.SetSelectable(true, false)
	if len(indexes) == 0 {
		idx.table.SetCell(0, 0, tview.NewTableCell("No indexes found").SetSelectable(false))
		return
	}

	headers := []string{"Name", "Columns", "Type", "Unique", "Primary", "Definition"}
	for i, h := range headers {
		idx.table.SetCell(0, i, tview.NewTableCell(" "+h+" ").
			SetSelectable(false).
			SetTextColor(styles.Content.ColumnKeyColor.Color()).
			SetBackgroundColor(styles.Content.HeaderRowBackgroundColor.Color()).
			SetAlign(tview.AlignCenter))
	}

	for r, ix := range indexes {
		unique := ""
		if ix.IsUnique {
			unique = "✓"
		}
		primary := ""
		if ix.IsPrimary {
			primary = "✓"
		}
		cols := strings.Join(ix.Columns, ", ")

		idx.table.SetCell(r+1, 0, tview.NewTableCell(" "+ix.Name+" ").
			SetTextColor(styles.Content.ColumnKeyColor.Color()).
			SetReference(ix.Name))
		idx.table.SetCell(r+1, 1, tview.NewTableCell(" "+cols+" ").
			SetTextColor(styles.Global.TextColor.Color()))
		idx.table.SetCell(r+1, 2, tview.NewTableCell(" "+ix.Type+" ").
			SetTextColor(styles.Global.SecondaryTextColor.Color()))
		idx.table.SetCell(r+1, 3, tview.NewTableCell(" "+unique+" ").
			SetTextColor(styles.Content.ColumnTypeColor.Color()).
			SetAlign(tview.AlignCenter))
		idx.table.SetCell(r+1, 4, tview.NewTableCell(" "+primary+" ").
			SetTextColor(styles.Content.ColumnTypeColor.Color()).
			SetAlign(tview.AlignCenter))
		idx.table.SetCell(r+1, 5, tview.NewTableCell(" "+ix.Definition+" ").
			SetTextColor(styles.Global.TextColor.Color()))
	}

	idx.table.Select(1, 0)
}

// showAddForm replaces the table with an inline form for creating a new index.
func (idx *Indexes) showAddForm() {
	idx.columnCount = 1
	idx.addForm.Clear(true)

	modeLabel := "Raw SQL"
	if idx.isRawMode {
		idx.addForm.AddInputField("SQL", "", 0, nil, nil)
		modeLabel = "Form"
	} else {
		idx.insertColumnPair(0, 1)
		idx.addForm.AddTextView("──────────────", "──────────────────────────────────────────────────", 0, 1, false, false)
		idx.addForm.AddInputField("Index Name", "", 30, nil, nil)
		idx.addForm.AddDropDown("Type", []string{"btree", "hash", "gin", "gist", "brin", "spgist"}, 0, nil)
		idx.addForm.AddCheckbox("Unique", false, nil)
		idx.addForm.AddButton("+Column", idx.addColumn)
	}

	idx.addForm.AddButton(modeLabel, func() {
		idx.isRawMode = !idx.isRawMode
		idx.showAddForm()
	})
	idx.addForm.AddButton("Create", idx.handleCreate)
	idx.addForm.AddButton("Cancel", idx.closeAddForm)

	idx.isAddFormVisible = true
	idx.Render()
	idx.App.SetFocus(idx.addForm)
}

func (idx *Indexes) insertColumnPair(pos, n int) {
	input := tview.NewInputField().
		SetLabel(fmt.Sprintf("Column %d", n)).
		SetFieldWidth(30)
	input.SetAutocompleteFunc(idx.autocompleteFunc)

	dropdown := tview.NewDropDown().
		SetLabel(fmt.Sprintf("Order %d", n)).
		SetOptions([]string{"ASC", "DESC"}, nil)

	idx.addForm.InsertFormItem(pos, input)
	idx.addForm.InsertFormItem(pos+1, dropdown)
}

func (idx *Indexes) autocompleteFunc(currentText string) []tview.AutocompleteItem {
	entries := make([]tview.AutocompleteItem, 0, len(idx.colKeys))
	for _, key := range idx.colKeys {
		if matched, _ := regexp.MatchString("(?i)^"+regexp.QuoteMeta(currentText), key); matched {
			entries = append(entries, tview.AutocompleteItem{Main: key})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Main) < strings.ToLower(entries[j].Main)
	})
	return entries
}

func (idx *Indexes) addColumn() {
	sepIdx := -1
	for i := 0; i < idx.addForm.GetFormItemCount(); i++ {
		if tv, ok := idx.addForm.GetFormItem(i).(*tview.TextView); ok && strings.Contains(tv.GetText(false), "──") {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		return
	}

	idx.columnCount++
	idx.insertColumnPair(sepIdx, idx.columnCount)
	idx.App.SetFocus(idx.addForm)
}

func (idx *Indexes) handleCreate() {
	ctx := context.Background()

	if idx.isRawMode {
		sql := idx.addForm.GetFormItemByLabel("SQL").(*tview.InputField).GetText()
		if strings.TrimSpace(sql) == "" {
			modal.ShowError(idx.App.Pages, "Validation error", fmt.Errorf("SQL statement is required"))
			return
		}
		if _, err := idx.Driver.ExecuteStatement(ctx, sql); err != nil {
			modal.ShowError(idx.App.Pages, "Error creating index", err)
			return
		}
		idx.closeAddForm()
		idx.loadData(ctx)
		return
	}

	var columns []string
	for i := 1; i <= idx.columnCount; i++ {
		item := idx.addForm.GetFormItemByLabel(fmt.Sprintf("Column %d", i))
		if item == nil {
			continue
		}
		col := item.(*tview.InputField).GetText()
		if col == "" {
			continue
		}
		dir := "ASC"
		if orderItem := idx.addForm.GetFormItemByLabel(fmt.Sprintf("Order %d", i)); orderItem != nil {
			if _, opt := orderItem.(*tview.DropDown).GetCurrentOption(); opt != "" {
				dir = opt
			}
		}
		columns = append(columns, col+" "+dir)
	}

	if len(columns) == 0 {
		modal.ShowError(idx.App.Pages, "Validation error", fmt.Errorf("at least one column is required"))
		return
	}

	name := idx.addForm.GetFormItemByLabel("Index Name").(*tview.InputField).GetText()
	_, indexType := idx.addForm.GetFormItemByLabel("Type").(*tview.DropDown).GetCurrentOption()
	unique := idx.addForm.GetFormItemByLabel("Unique").(*tview.Checkbox).IsChecked()

	def := database.IndexDefinition{
		Name:     name,
		Columns:  columns,
		IsUnique: unique,
		Type:     indexType,
	}

	if err := idx.Driver.CreateIndex(ctx, idx.schema, idx.tbl, def); err != nil {
		modal.ShowError(idx.App.Pages, "Error creating index", err)
		return
	}

	idx.closeAddForm()
	idx.loadData(ctx)
}

func (idx *Indexes) closeAddForm() {
	idx.addForm.Clear(true)
	idx.isAddFormVisible = false
	idx.Render()
	idx.App.SetFocus(idx)
}

func (idx *Indexes) showDeleteIndexModal(ctx context.Context) {
	row, _ := idx.table.GetSelection()
	if row < 1 {
		return
	}
	cell := idx.table.GetCell(row, 0)
	indexName, _ := cell.GetReference().(string)
	if indexName == "" {
		return
	}

	idx.deleteModal.SetConfirmButtonLabel("Drop")
	idx.deleteModal.SetText(fmt.Sprintf("Drop index [::b]%s[-:-:-]?", indexName))
	idx.deleteModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		defer idx.App.Pages.RemovePage(IndexDeleteModalId)
		if buttonIndex == 0 {
			err := idx.Driver.DropIndex(ctx, idx.schema, indexName)
			if err != nil {
				modal.ShowError(idx.App.Pages, "Error dropping index", err)
				return
			}
			idx.table.RemoveRow(row)
			idx.table.Select(row-1, 0)
		}
	})

	idx.App.Pages.AddPage(IndexDeleteModalId, idx.deleteModal, true, true)
}

func (idx *Indexes) IsAddFormFocused() bool {
	return idx.isAddFormVisible
}
