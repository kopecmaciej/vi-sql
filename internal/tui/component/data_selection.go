package component

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
)

func (c *Data) updateSelectionTitle() {
	if c.resultGrid.cellSelection {
		c.tableFlex.SetTitle(fmt.Sprintf(" Table — SELECT %d cells · c: change · Esc: cancel ", len(c.resultGrid.SelectedCells())))
	} else if c.mode == TableMode {
		c.tableFlex.SetTitle(" Table ")
	}
}

func (c *Data) handleCellSelection(ctx context.Context, event *tcell.EventKey) *tcell.EventKey {
	k := c.App.GetKeys()
	row, col := c.resultGrid.GetSelection()
	isRune := func(r rune) bool {
		return event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone && event.Rune() == r && !k.HasPending()
	}
	// Normal key mode has no sequence prefix handler; retain gg in selection mode.
	if isRune('g') {
		if !c.selectionPendingTop {
			c.selectionPendingTop = true
			return nil
		}
		c.selectionPendingTop = false
		c.resultGrid.Select(1, col)
		c.updateSelectionTitle()
		return nil
	}
	c.selectionPendingTop = false
	switch {
	case k.Match(k.Data.ClearSelection, event):
		c.resultGrid.ClearSelection()
	case k.Match(k.Data.ChangeSelection, event):
		c.handleBatchEdit(ctx)
		return nil
	case k.Match(k.Navigation.GoTop, event):
		row = 1
	case k.Match(k.Navigation.GoBottom, event), isRune('G'):
		row = c.resultGrid.GetRowCount() - 1
	case k.Match(k.Navigation.MoveUp, event), isRune('k'):
		row--
	case k.Match(k.Navigation.MoveDown, event), isRune('j'):
		row++
	case k.Match(k.Navigation.MoveLeft, event), isRune('h'):
		col--
	case k.Match(k.Navigation.MoveRight, event), isRune('l'):
		col++
	default:
		// Keep the displayed rows and columns stable until the selection is finished.
		return nil
	}
	c.resultGrid.Select(max(1, min(row, c.resultGrid.GetRowCount()-1)), max(1, min(col, c.resultGrid.GetColumnCount()-1)))
	c.updateSelectionTitle()
	return nil
}

// selectedRowUpdates snapshots visible cells before opening the editor. Primary
// keys identify the original rows even when columns are hidden or rows filtered.
func (c *Data) selectedRowUpdates() ([]database.RowUpdate, []string, error) {
	pkCols := c.state.GetPrimaryKey()
	if len(pkCols) == 0 {
		return nil, nil, fmt.Errorf("batch editing requires a primary key")
	}
	rows := c.search.filtered(c)
	var updates []database.RowUpdate
	var columns []string
	lastRow := -1
	for _, cell := range c.resultGrid.SelectedCells() {
		name := c.resultGrid.ColumnName(cell.Col)
		if name == "" || name == "_pk" || slices.Contains(pkCols, name) {
			return nil, nil, fmt.Errorf("primary key and internal columns cannot be batch edited; select only editable cells")
		}
		if !slices.Contains(columns, name) {
			columns = append(columns, name)
		}
		if cell.Row == lastRow {
			continue
		}
		original := c.resultGrid.RowData(cell.Row, rows)
		pk := c.resultGrid.RowPrimaryKey(cell.Row, rows, pkCols)
		if original == nil || pk == nil {
			return nil, nil, fmt.Errorf("selected row is no longer available")
		}
		for _, name := range pkCols {
			if value, ok := original[name]; !ok || value == nil {
				return nil, nil, fmt.Errorf("selected row has an incomplete primary key")
			}
		}
		updates = append(updates, database.RowUpdate{PrimaryKey: *pk, Original: maps.Clone(original)})
		lastRow = cell.Row
	}
	return updates, columns, nil
}

func (c *Data) handleBatchEdit(ctx context.Context) {
	updates, columns, err := c.selectedRowUpdates()
	if err != nil {
		modal.ShowError(c.App.Pages, "Cannot edit selection", err)
		return
	}
	if len(updates) == 0 || len(columns) == 0 {
		return
	}
	row, col := c.resultGrid.GetSelection()
	closeEditor := func() {
		c.inlineEdit.Hide()
		c.resultGrid.ClearSelection()
		c.updateSelectionTitle()
		c.App.SetFocus(c.resultGrid)
	}
	c.inlineEdit.SetCancelCallback(closeEditor)
	c.inlineEdit.SetApplyCallback(func(_ string, value string) error {
		for i := range updates {
			updates[i].Updated = maps.Clone(updates[i].Original)
			for _, name := range columns {
				if value != c.App.GetFormatter().EditableString(updates[i].Original[name]) {
					updates[i].Updated[name] = value
				}
			}
		}
		if err := c.Driver.UpdateRows(ctx, c.state.Schema, c.state.Table, updates); err != nil {
			return err
		}
		for _, update := range updates {
			c.state.UpdateRow(update.PrimaryKey, update.Updated)
		}
		closeEditor()
		c.reRenderState()
		c.resultGrid.Select(min(row, c.resultGrid.GetRowCount()-1), c.resultGrid.ClampCol(col))
		return nil
	})
	currentValue := c.App.GetFormatter().EditableString(updates[0].Original[columns[0]])
	c.inlineEdit.RenderBatch(len(updates)*len(columns), currentValue)
}
