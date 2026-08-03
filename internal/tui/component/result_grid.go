package component

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

// ResultGrid wraps *core.Table and owns all rendering and selection logic so
// callers don't need to parse cell header references or manage row offsets.
type ResultGrid struct {
	*core.Table
	app                *core.App
	hiddenCols         []string
	searchHighlightHex string
}

func NewResultGrid() *ResultGrid {
	return &ResultGrid{Table: core.NewTable()}
}

func (g *ResultGrid) SetApp(app *core.App) { g.app = app }

// HideColumn adds the column at col to the hidden list and returns its name.
// Returns "" if col has no header reference (no-op).
func (g *ResultGrid) HideColumn(col int) string {
	name := g.ColumnName(col)
	if name == "" {
		return ""
	}
	g.hiddenCols = append(g.hiddenCols, name)
	return name
}

// ResetHiddenColumns clears the hidden column list.
func (g *ResultGrid) ResetHiddenColumns() {
	g.hiddenCols = nil
}

func (g *ResultGrid) SetStyle(styles *config.Styles, dataStyle *config.DataStyle) {
	g.Table.SetStyle(styles)
	g.SetBordersColor(styles.Others.SeparatorColor.Color())
	g.SetSeparator(styles.Icons.Separator.Rune())
	g.SetMultiSelectedStyle(tcell.StyleDefault.
		Background(dataStyle.MultiSelectedRowColor.Color()).
		Foreground(tcell.ColorWhite))
	g.searchHighlightHex = dataStyle.SearchHighlightColor.String()
}

// ColumnName returns the column name stored in the header-cell reference for col.
// Returns "" if the header cell is absent or has no string reference.
func (g *ResultGrid) ColumnName(col int) string {
	cell := g.GetCell(0, col)
	if cell == nil {
		return ""
	}
	name, _ := cell.GetReference().(string)
	return name
}

// RowData returns the row map for the given table row index (1-indexed, row 0
// is the header). Returns nil if out of range.
func (g *ResultGrid) RowData(row int, allRows []database.Row) database.Row {
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(allRows) {
		return nil
	}
	return allRows[dataRow]
}

// RowPrimaryKey builds a PrimaryKey for the given row. Returns nil if pkCols
// is empty or row is out of range.
func (g *ResultGrid) RowPrimaryKey(row int, allRows []database.Row, pkCols []string) *database.PrimaryKey {
	if len(pkCols) == 0 {
		return nil
	}
	rowData := g.RowData(row, allRows)
	if rowData == nil {
		return nil
	}
	pk := database.PrimaryKey{Columns: make(map[string]any)}
	for _, col := range pkCols {
		pk.Columns[col] = rowData[col]
	}
	return &pk
}

// VisibleColumns returns column names in ordinal order, excluding hidden ones.
func (g *ResultGrid) VisibleColumns(row database.Row, cols []database.ColumnInfo) []string {
	all := orderedColumnNames(row, cols)
	var visible []string
	for _, col := range all {
		if !slices.Contains(g.hiddenCols, col) {
			visible = append(visible, col)
		}
	}
	return visible
}

// FindMatches returns grid (row, col) coordinates of cells containing the
// search text within filteredRows. Coordinates are 1-indexed: row 0 is the
// header row and col 0 is the fixed "#" column. Returns nil when text is
// empty or no rows match.
func (g *ResultGrid) FindMatches(searchText string, filteredRows []database.Row, cols []database.ColumnInfo) [][2]int {
	if searchText == "" || len(filteredRows) == 0 {
		return nil
	}
	lowerText := strings.ToLower(searchText)
	visibleCols := g.VisibleColumns(filteredRows[0], cols)

	var matches [][2]int
	for i, row := range filteredRows {
		for j, colName := range visibleCols {
			if strings.Contains(strings.ToLower(database.StringifyValue(row[colName])), lowerText) {
				matches = append(matches, [2]int{i + 1, j + 1})
			}
		}
	}
	return matches
}

// ClampCol returns col clamped to the valid column range [0, columnCount-1].
func (g *ResultGrid) ClampCol(col int) int {
	if count := g.GetColumnCount(); col >= count {
		col = count - 1
	}
	if col < 0 {
		col = 0
	}
	return col
}

// FlashCell briefly highlights a single cell to confirm a copy action.
func (g *ResultGrid) FlashCell(row, col int) {
	if cell := g.GetCell(row, col); cell != nil {
		g.flashCells([]*tview.TableCell{cell})
	}
}

// FlashRow briefly highlights all cells in a row to confirm a copy action.
func (g *ResultGrid) FlashRow(row int) {
	numCols := g.GetColumnCount()
	cells := make([]*tview.TableCell, 0, numCols)
	for col := range numCols {
		if cell := g.GetCell(row, col); cell != nil {
			cells = append(cells, cell)
		}
	}
	g.flashCells(cells)
}

// CopyCell copies the value at (row, col) to the clipboard and flashes the cell.
func (g *ResultGrid) CopyCell(row, col int, allRows []database.Row) bool {
	colName := g.ColumnName(col)
	if colName == "" {
		return false
	}
	rowData := g.RowData(row, allRows)
	if rowData == nil {
		return false
	}
	util.Copy(database.StringifyValue(rowData[colName]))
	g.FlashCell(row, col)
	return true
}

// CopyRow copies visable columns of selected rows and flashed those rows.
func (g *ResultGrid) CopyRow(row int, allRows []database.Row, cols []database.ColumnInfo) bool {
	if len(allRows) == 0 {
		return false
	}
	visibleCols := g.VisibleColumns(allRows[0], cols)

	rowIndices := g.GetSelectedRows()
	if len(rowIndices) == 0 {
		if row < 1 {
			return false
		}
		rowIndices = []int{row}
	}

	var lines []string
	for _, r := range rowIndices {
		rowData := g.RowData(r, allRows)
		if rowData == nil {
			continue
		}
		var parts []string
		for _, col := range visibleCols {
			parts = append(parts, fmt.Sprintf("%s: %s", col, database.StringifyValue(rowData[col])))
		}
		lines = append(lines, strings.Join(parts, ", "))
	}
	if len(lines) == 0 {
		return false
	}
	util.Copy(strings.Join(lines, "\n"))
	for _, r := range rowIndices {
		g.FlashRow(r)
	}
	return true
}

// CopyRowAs formats the selected row(s) as the given export format and copies
// to clipboard. JSON with a single row is unwrapped to an object; multiple
// rows are a JSON array. CSV includes a header row.
func (g *ResultGrid) CopyRowAs(format util.ExportFormat, row int, allRows []database.Row, cols []database.ColumnInfo) bool {
	if len(allRows) == 0 {
		return false
	}
	visibleCols := g.VisibleColumns(allRows[0], cols)

	rowIndices := g.GetSelectedRows()
	if len(rowIndices) == 0 {
		if row < 1 {
			return false
		}
		rowIndices = []int{row}
	}

	var rows []map[string]any
	for _, r := range rowIndices {
		if data := g.RowData(r, allRows); data != nil {
			rows = append(rows, data)
		}
	}
	if len(rows) == 0 {
		return false
	}

	var sb strings.Builder
	var err error
	if format == util.ExportJSON && len(rows) == 1 {
		obj := make(map[string]any, len(visibleCols))
		for _, col := range visibleCols {
			obj[col] = util.AsJSONValue(rows[0][col])
		}
		var enc []byte
		enc, err = json.MarshalIndent(obj, "", "  ")
		if err == nil {
			sb.Write(enc)
		}
	} else {
		err = util.ExportRows(&sb, format, visibleCols, rows, "", "", util.ExportOptions{
			IncludeHeaders: format == util.ExportCSV,
			PrettyPrint:    format == util.ExportJSON,
		})
	}
	if err != nil {
		return false
	}

	util.Copy(sb.String())
	for _, r := range rowIndices {
		g.FlashRow(r)
	}
	return true
}

func (g *ResultGrid) flashCells(cells []*tview.TableCell) {
	flashBg := g.app.GetStyles().Global.MoreContrastBackgroundColor.Color()
	for _, cell := range cells {
		cell.SetBackgroundColor(flashBg)
	}
	go func() {
		time.Sleep(350 * time.Millisecond)
		g.app.QueueUpdateDraw(func() {
			for _, cell := range cells {
				cell.SetTransparency(true)
			}
		})
	}()
}

// Render paints the header row and all data rows. The table must be cleared
// before calling this. searchText is the substring to highlight within cell values.
func (g *ResultGrid) Render(rows []database.Row, cols []database.ColumnInfo, styles *config.Styles, searchText string) {
	// Column 0 is a fixed row-number column (#); data columns start at 1.
	g.SetOffset(0, 0)
	g.SetFixed(1, 1)
	g.SetSelectable(true, true)

	g.SetCell(0, 0, tview.NewTableCell("#").
		SetSelectable(false).
		SetBackgroundColor(styles.Global.ContrastBackgroundColor.Color()).
		SetAlign(tview.AlignCenter))

	visibleCols := g.VisibleColumns(rows[0], cols)

	typeMap := make(map[string]string)
	boolCols := make(map[string]bool)
	pkCols := make(map[string]bool)
	fkCols := make(map[string]bool)
	for _, col := range cols {
		typeMap[col.Name] = styles.Icons.TypeSymbol(col.DataType)
		if col.DataType == "boolean" {
			boolCols[col.Name] = true
		}
		if col.IsPK {
			pkCols[col.Name] = true
		}
		if col.IsFK {
			fkCols[col.Name] = true
		}
	}

	for col, name := range visibleCols {
		headerText := name
		if t, ok := typeMap[name]; ok {
			pkPrefix := ""
			if pkCols[name] {
				pkPrefix = styles.Icons.IconWithColor(styles.Icons.PrimaryKey, styles.Global.SecondaryTextColor)
			}
			fkPrefix := ""
			if fkCols[name] {
				fkPrefix = styles.Icons.IconWithColor(styles.Icons.ForeignKey, styles.SQLEditor.StringColor)
			}
			headerText = fmt.Sprintf("%s%s[%s]%s [%s]%s ",
				pkPrefix, fkPrefix,
				styles.Global.SecondaryTextColor.String(), name,
				styles.Global.MoreContrastBackgroundColor.String(), t)
		}
		g.SetCell(0, col+1, tview.NewTableCell(headerText).
			SetReference(name).
			SetSelectable(false).
			SetBackgroundColor(styles.Global.ContrastBackgroundColor.Color()).
			SetAlign(tview.AlignCenter))
	}

	for row, rowData := range rows {
		g.SetCell(row+1, 0, tview.NewTableCell(fmt.Sprintf("[%s]%d[-]", styles.Global.DimColor, row+1)).
			SetSelectable(false).
			SetAlign(tview.AlignRight).
			SetMaxWidth(6))
		for col, colName := range visibleCols {
			isNull := rowData[colName] == nil
			cellText := database.StringifyValue(rowData[colName])
			if boolCols[colName] {
				switch cellText {
				case "t":
					cellText = "true"
				case "f":
					cellText = "false"
				}
			}
			if len(cellText) > 35 {
				cellText = cellText[:35] + "..."
			}
			if isNull {
				cellText = fmt.Sprintf("[%s]NULL[-:-:-]", styles.Global.DimColor)
			} else {
				cellText = tview.Escape(cellText)
				if searchText != "" {
					cellText = highlightMatches(cellText, searchText, g.searchHighlightHex)
				}
			}
			g.SetCell(row+1, col+1, tview.NewTableCell(cellText).
				SetAlign(tview.AlignLeft).
				SetMaxWidth(30))
		}
	}
	g.Select(1, 1)
}

// orderedColumnNames returns column names in ordinal_position order using cols
// metadata. Falls back to alphabetical if metadata is absent.
func orderedColumnNames(row database.Row, cols []database.ColumnInfo) []string {
	if len(cols) > 0 {
		names := make([]string, 0, len(cols))
		for _, col := range cols {
			if _, ok := row[col.Name]; ok {
				names = append(names, col.Name)
			}
		}
		return names
	}
	return database.GetSortedColumnNames(row)
}

func highlightMatches(text, search, highlightHex string) string {
	lower := strings.ToLower(text)
	searchLower := strings.ToLower(search)
	highlightOpen := fmt.Sprintf("[:%s]", highlightHex)
	highlightClose := "[-:-:-]"

	var result strings.Builder
	last := 0
	for {
		idx := strings.Index(lower[last:], searchLower)
		if idx == -1 {
			break
		}
		idx += last
		result.WriteString(text[last:idx])
		result.WriteString(highlightOpen)
		result.WriteString(text[idx : idx+len(search)])
		result.WriteString(highlightClose)
		last = idx + len(search)
	}
	result.WriteString(text[last:])
	return result.String()
}
