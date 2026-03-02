package component

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
)

const (
	IndexId           = "Index"
	IndexDeleteModalId = "IndexDeleteModal"
	IndexInputModalId  = "IndexInputModal"
)

// Indexes displays index information for the currently selected table and
// allows creating and dropping indexes.
type Indexes struct {
	*core.BaseElement
	*core.Flex

	innerFlex    *core.Flex
	table        *core.Table
	confirmModal *modal.Confirm
	inputModal   *primitives.InputModal

	schema  string
	tbl     string
	indexes []database.IndexInfo
}

func NewIndexes() *Indexes {
	idx := &Indexes{
		BaseElement:  core.NewBaseElement(),
		Flex:         core.NewFlex(),
		innerFlex:    core.NewFlex(),
		table:        core.NewTable(),
		confirmModal: modal.NewConfirm(IndexDeleteModalId),
		inputModal:   primitives.NewInputModal(),
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

	if err := idx.confirmModal.Init(idx.App); err != nil {
		return err
	}

	idx.handleEvents()
	return nil
}

func (idx *Indexes) setStyle() {
	styles := idx.App.GetStyles()
	idx.Flex.SetStyle(styles)
	idx.innerFlex.SetStyle(styles)
	idx.table.SetStyle(styles)
	idx.innerFlex.SetBorderColor(styles.Others.SeparatorColor.Color())
	idx.table.SetBordersColor(styles.Others.SeparatorColor.Color())

	idx.inputModal.SetBorderColor(styles.Global.BorderColor.Color())
	idx.inputModal.SetBackgroundColor(styles.Global.BackgroundColor.Color())
	idx.inputModal.SetFieldTextColor(styles.Others.ModalTextColor.Color())
	idx.inputModal.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
}

func (idx *Indexes) setLayout() {
	idx.Flex.SetDirection(tview.FlexRow)

	idx.innerFlex.SetBorder(true)
	idx.innerFlex.SetTitle(" Indexes ")
	idx.innerFlex.SetTitleAlign(tview.AlignCenter)
	idx.innerFlex.SetBorderPadding(0, 0, 1, 1)
	idx.innerFlex.SetDirection(tview.FlexRow)

	idx.inputModal.SetBorder(true)
	idx.inputModal.SetTitle(" Add Index ")
}

func (idx *Indexes) setKeybindings() {
	k := idx.App.GetKeys()
	ctx := context.Background()

	idx.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Index.AddIndex, event.Name()):
			idx.showAddModal(ctx)
			return nil
		case k.Contains(k.Index.DeleteIndex, event.Name()):
			idx.showDeleteModal(ctx)
			return nil
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
	idx.innerFlex.Clear()
	idx.innerFlex.AddItem(idx.table, 0, 1, true)
	idx.Flex.AddItem(idx.innerFlex, 0, 1, true)
}

// HandleTableSelection loads index data for the given schema/table.
func (idx *Indexes) HandleTableSelection(ctx context.Context, schema, table string) {
	idx.schema = schema
	idx.tbl = table
	idx.loadData(ctx)
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
		def := ix.Definition
		if len(def) > 40 {
			def = def[:40] + "..."
		}

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
		idx.table.SetCell(r+1, 5, tview.NewTableCell(" "+def+" ").
			SetTextColor(styles.Global.TextColor.Color()))
	}

	idx.table.Select(1, 0)
}

func (idx *Indexes) showAddModal(ctx context.Context) {
	template := fmt.Sprintf("CREATE INDEX idx_%s_ ON %s.%s (col)", idx.tbl, idx.schema, idx.tbl)
	idx.inputModal.SetText(template)
	idx.inputModal.SetLabel(fmt.Sprintf("Create index on [::b]%s.%s", idx.schema, idx.tbl))

	idx.inputModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			sql := strings.TrimSpace(idx.inputModal.GetText())
			if sql == "" {
				return event
			}
			_, err := idx.Driver.ExecuteStatement(ctx, sql)
			if err != nil {
				modal.ShowError(idx.App.Pages, "Error creating index", err)
			} else {
				idx.loadData(ctx)
			}
			idx.closeAddModal()
		case tcell.KeyEscape:
			idx.closeAddModal()
		}
		return event
	})

	idx.App.Pages.AddPage(IndexInputModalId, idx.inputModal, true, true)
}

func (idx *Indexes) closeAddModal() {
	idx.inputModal.SetText("")
	idx.App.Pages.RemovePage(IndexInputModalId)
}

func (idx *Indexes) showDeleteModal(ctx context.Context) {
	row, _ := idx.table.GetSelection()
	if row < 1 {
		return
	}

	cell := idx.table.GetCell(row, 0)
	if cell == nil {
		return
	}
	indexName, _ := cell.GetReference().(string)
	if indexName == "" {
		return
	}

	idx.confirmModal.SetConfirmButtonLabel("Drop")
	idx.confirmModal.SetText(fmt.Sprintf("Drop index [::b]%s[-:-:-]?", indexName))
	idx.confirmModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		defer idx.App.Pages.RemovePage(IndexDeleteModalId)
		if buttonLabel == "Drop" {
			err := idx.Driver.DropIndex(ctx, idx.schema, indexName)
			if err != nil {
				modal.ShowError(idx.App.Pages, "Error dropping index", err)
				return
			}
			idx.loadData(ctx)
		}
	})

	idx.App.Pages.AddPage(IndexDeleteModalId, idx.confirmModal, true, true)
}
