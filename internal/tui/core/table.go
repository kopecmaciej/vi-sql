package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
)

type Table struct {
	*tview.Table
}

func NewTable() *Table {
	return &Table{
		Table: tview.NewTable(),
	}
}

func (t *Table) SetStyle(style *config.Styles) {
	t.SetBackgroundColor(style.Global.BackgroundColor.Color())
	t.SetBorderColor(style.Global.BorderColor.Color())
	t.SetTitleColor(style.Global.TitleColor.Color())
	t.SetFocusStyle(tcell.StyleDefault.
		Foreground(style.Global.FocusColor.Color()).
		Background(style.Global.BackgroundColor.Color()))
}

// MoveUpUntil moves the selection up until a condition is met.
func (t *Table) MoveUpUntil(row, col int, condition func(cell *tview.TableCell) bool) {
	for row > 0 {
		row--
		cell := t.GetCell(row, col)
		if condition(cell) {
			t.Select(row, col)
			return
		}
	}
}

// MoveDownUntil moves the selection down until a condition is met.
func (t *Table) MoveDownUntil(row, col int, condition func(cell *tview.TableCell) bool) {
	for row < t.GetRowCount()-1 {
		row++
		cell := t.GetCell(row, col)
		if condition(cell) {
			t.Select(row, col)
			return
		}
	}
}

// GetContentFromRows returns the content of the table from the selected rows.
func (t *Table) GetContentFromRows(rows []int) []string {
	content := []string{}
	for _, row := range rows {
		content = append(content, t.GetCell(row, 0).GetReference().(string))
	}
	return content
}
