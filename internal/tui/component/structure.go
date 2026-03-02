package component

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
)

const StructureId = "Structure"

// Structure displays column definitions, constraints, and foreign keys for the
// currently selected table.
type Structure struct {
	*core.BaseElement
	*core.Flex

	innerFlex *core.Flex
	table     *core.Table

	schema string
	tbl    string
}

func NewStructure() *Structure {
	s := &Structure{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		innerFlex:   core.NewFlex(),
		table:       core.NewTable(),
	}

	s.SetIdentifier(StructureId)
	s.table.SetIdentifier(StructureId)
	s.SetAfterInitFunc(s.init)

	return s
}

func (s *Structure) init() error {
	s.setStyle()
	s.setLayout()
	s.setKeybindings()
	s.handleEvents()
	return nil
}

func (s *Structure) setStyle() {
	styles := s.App.GetStyles()
	s.Flex.SetStyle(styles)
	s.innerFlex.SetStyle(styles)
	s.table.SetStyle(styles)
	s.innerFlex.SetBorderColor(styles.Others.SeparatorColor.Color())
	s.table.SetBordersColor(styles.Others.SeparatorColor.Color())
}

func (s *Structure) setLayout() {
	s.Flex.SetDirection(tview.FlexRow)

	s.innerFlex.SetBorder(true)
	s.innerFlex.SetTitle(" Structure ")
	s.innerFlex.SetTitleAlign(tview.AlignCenter)
	s.innerFlex.SetBorderPadding(0, 0, 1, 1)
	s.innerFlex.SetDirection(tview.FlexRow)
}

func (s *Structure) setKeybindings() {
	k := s.App.GetKeys()
	s.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if k.Contains(k.Structure.Refresh, event.Name()) {
			s.loadData(context.Background())
			return nil
		}
		return event
	})
}

func (s *Structure) handleEvents() {
	go s.HandleEvents(StructureId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			s.setStyle()
			s.App.QueueUpdateDraw(func() {
				s.Render()
			})
		}
	})
}

func (s *Structure) Render() {
	s.Flex.Clear()
	s.innerFlex.Clear()
	s.innerFlex.AddItem(s.table, 0, 1, true)
	s.Flex.AddItem(s.innerFlex, 0, 1, true)
}

// HandleTableSelection loads structure data for the given schema/table.
func (s *Structure) HandleTableSelection(ctx context.Context, schema, table string) {
	s.schema = schema
	s.tbl = table
	s.loadData(ctx)
}

func (s *Structure) loadData(ctx context.Context) {
	if s.schema == "" || s.tbl == "" {
		return
	}

	columns, err := s.Driver.GetTableColumns(ctx, s.schema, s.tbl)
	if err != nil {
		modal.ShowError(s.App.Pages, "Error loading columns", err)
		return
	}

	constraints, _ := s.Driver.GetTableConstraints(ctx, s.schema, s.tbl)
	fks, _ := s.Driver.GetTableForeignKeys(ctx, s.schema, s.tbl)

	pkCols := map[string]bool{}
	fkCols := map[string]string{}

	for _, c := range constraints {
		if c.Type == "PRIMARY KEY" {
			for _, col := range c.Columns {
				pkCols[col] = true
			}
		}
	}
	for _, fk := range fks {
		for _, col := range fk.Columns {
			fkCols[col] = fk.ReferencedTable
		}
	}

	s.renderColumns(columns, pkCols, fkCols)
}

func (s *Structure) renderColumns(columns []database.ColumnInfo, pkCols map[string]bool, fkCols map[string]string) {
	styles := s.App.GetStyles()
	s.table.Clear()
	s.table.SetFixed(1, 0)
	s.table.SetSelectable(true, false)

	headers := []string{"Column", "Type", "Nullable", "Default", "Constraints"}
	for i, h := range headers {
		s.table.SetCell(0, i, tview.NewTableCell(" "+h+" ").
			SetSelectable(false).
			SetTextColor(styles.Content.ColumnKeyColor.Color()).
			SetBackgroundColor(styles.Content.HeaderRowBackgroundColor.Color()).
			SetAlign(tview.AlignCenter))
	}

	for r, col := range columns {
		nullable := "NOT NULL"
		if col.IsNullable {
			nullable = "NULL"
		}

		def := ""
		if col.Default != nil {
			def = *col.Default
			if len(def) > 25 {
				def = def[:25] + "..."
			}
		}

		var constraints []string
		if pkCols[col.Name] {
			constraints = append(constraints, "PK")
		}
		if ref, ok := fkCols[col.Name]; ok {
			constraints = append(constraints, "FK→"+ref)
		}

		s.table.SetCell(r+1, 0, tview.NewTableCell(" "+col.Name+" ").
			SetTextColor(styles.Content.ColumnKeyColor.Color()))
		s.table.SetCell(r+1, 1, tview.NewTableCell(" "+col.DataType+" ").
			SetTextColor(styles.Global.TextColor.Color()))
		s.table.SetCell(r+1, 2, tview.NewTableCell(" "+nullable+" ").
			SetTextColor(styles.Global.SecondaryTextColor.Color()))
		s.table.SetCell(r+1, 3, tview.NewTableCell(" "+def+" ").
			SetTextColor(styles.Global.TextColor.Color()))
		s.table.SetCell(r+1, 4, tview.NewTableCell(" "+strings.Join(constraints, ", ")+" ").
			SetTextColor(styles.Content.ColumnTypeColor.Color()))
	}

	if len(columns) > 0 {
		s.table.Select(1, 0)
	}
}
