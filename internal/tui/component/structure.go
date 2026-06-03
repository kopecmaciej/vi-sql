package component

import (
	"context"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/rs/zerolog/log"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const StructureId = "Structure"

// Structure displays column definitions, constraints, and foreign keys for the
// currently selected table, with a read-only DDL pane below.
type Structure struct {
	*core.BaseElement
	*core.Flex

	innerFlex  *core.Flex
	table      *core.Table
	ddlView    *core.TextView
	inlineEdit *modal.InlineEditModal

	schema  string
	tbl     string
	columns []database.ColumnInfo
	pkCols  map[string]bool
	fkCols  map[string]string
	ddl     string
	showDDL bool
}

func NewStructure() *Structure {
	s := &Structure{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		innerFlex:   core.NewFlex(),
		table:       core.NewTable(),
		ddlView:     core.NewTextView(),
		inlineEdit:  modal.NewInlineEditModal(),
		showDDL:     true,
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
	return s.inlineEdit.Init(s.App)
}

func (s *Structure) setStyle() {
	styles := s.App.GetStyles()
	s.Flex.SetStyle(styles)
	s.innerFlex.SetStyle(styles)
	s.table.SetStyle(styles)
	s.ddlView.SetStyle(styles)
	s.innerFlex.SetBorderColor(styles.Others.SeparatorColor.Color())
	s.table.SetBordersColor(styles.Others.SeparatorColor.Color())
	s.ddlView.SetBorderColor(styles.Others.SeparatorColor.Color())
}

func (s *Structure) setLayout() {
	s.Flex.SetDirection(tview.FlexRow)

	s.innerFlex.SetBorder(true)
	s.innerFlex.SetTitle(" Structure ")
	s.innerFlex.SetTitleAlign(tview.AlignCenter)
	s.innerFlex.SetBorderPadding(0, 0, 1, 1)
	s.innerFlex.SetDirection(tview.FlexRow)

	s.ddlView.SetBorder(true)
	s.ddlView.SetTitle(" DDL ")
	s.ddlView.SetTitleAlign(tview.AlignCenter)
	s.ddlView.SetBorderPadding(0, 0, 1, 1)
	s.ddlView.SetDynamicColors(true)
	s.ddlView.SetScrollable(true)
	s.ddlView.SetWrap(false)
}

func (s *Structure) setKeybindings() {
	vimMode := s.App.GetConfig().UI.VimMode
	s.table.SetVimKeys(vimMode)
	s.ddlView.SetVimKeys(vimMode)
	k := s.App.GetKeys()
	s.table.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Common.Refresh, event):
			s.loadData(context.Background(), false)
			return nil
		case k.Match(k.Structure.RenameColumn, event):
			s.handleRenameColumn(context.Background())
			return nil
		case k.Match(k.Structure.ToggleDDLPane, event):
			s.showDDL = !s.showDDL
			s.Render()
			return nil
		case s.showDDL && k.Match(k.Navigation.FocusDown, event):
			s.App.SetFocusOnly(s.ddlView)
			return nil
		}
		return event
	}))

	s.ddlView.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.FocusUp, event):
			s.App.SetFocusOnly(s.table)
			return nil
		case k.Match(k.Common.Copy, event):
			util.Copy(s.ddl)
			return nil
		case k.Match(k.Structure.ToggleDDLPane, event):
			s.showDDL = !s.showDDL
			s.Render()
			s.App.SetFocusOnly(s.table)
			return nil
		}
		return event
	}))
}

func (s *Structure) handleRenameColumn(ctx context.Context) {
	row, _ := s.table.GetSelection()
	if row < 1 || row-1 >= len(s.columns) {
		return
	}
	colName := s.columns[row-1].Name

	s.inlineEdit.SetApplyCallback(func(_, newName string) error {
		if newName == "" || newName == colName {
			s.inlineEdit.Hide()
			s.App.SetFocus(s.table)
			return nil
		}
		if err := s.Driver.RenameColumn(ctx, s.schema, s.tbl, colName, newName); err != nil {
			return err
		}
		s.inlineEdit.Hide()
		s.App.SetFocus(s.table)
		s.loadData(ctx, false)
		return nil
	})
	s.inlineEdit.SetCancelCallback(func() {
		s.inlineEdit.Hide()
		s.App.SetFocus(s.table)
	})
	s.inlineEdit.Render(colName, colName)
}

func (s *Structure) handleEvents() {
	go s.HandleEvents(StructureId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			s.setStyle()
			s.Render()
			s.loadData(context.Background(), true)
		}
	})
}

func (s *Structure) Render() {
	s.Flex.Clear()
	s.innerFlex.Clear()
	s.innerFlex.AddItem(s.table, 0, 1, true)
	if s.showDDL {
		s.Flex.AddItem(s.innerFlex, 0, 2, true)
		s.Flex.AddItem(s.ddlView, 0, 1, false)
	} else {
		s.Flex.AddItem(s.innerFlex, 0, 1, true)
	}
}

// HandleTableSelection loads structure data for the given schema/table.
func (s *Structure) HandleTableSelection(ctx context.Context, schema, table string) {
	s.schema = schema
	s.tbl = table
	s.columns = nil
	s.loadData(ctx, false)
}

func (s *Structure) loadData(ctx context.Context, useState bool) {
	if s.schema == "" || s.tbl == "" {
		return
	}

	if useState && s.columns != nil {
		s.renderColumns(s.columns, s.pkCols, s.fkCols)
		s.renderDDL()
		return
	}

	columns, err := s.Driver.GetTableColumns(ctx, s.schema, s.tbl)
	if err != nil {
		modal.ShowError(s.App.Pages, "Error loading columns", err)
		return
	}

	constraints, err := s.Driver.GetTableConstraints(ctx, s.schema, s.tbl)
	if err != nil {
		log.Warn().Err(err).Str("table", s.tbl).Msg("Failed to load table constraints")
	}
	fks, err := s.Driver.GetTableForeignKeys(ctx, s.schema, s.tbl)
	if err != nil {
		log.Warn().Err(err).Str("table", s.tbl).Msg("Failed to load foreign keys")
	}

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

	s.columns = columns
	s.pkCols = pkCols
	s.fkCols = fkCols

	s.renderColumns(columns, pkCols, fkCols)

	ddl, err := s.Driver.GetTableDDL(ctx, s.schema, s.tbl)
	if err != nil {
		log.Warn().Err(err).Str("table", s.tbl).Msg("Failed to load table DDL")
	} else {
		s.ddl = ddl
		s.renderDDL()
	}
}

func (s *Structure) renderDDL() {
	if s.ddl == "" {
		return
	}
	styles := s.App.GetStyles()
	s.ddlView.SetText(core.ColorizeSQLText(s.ddl, &styles.SQLEditor))
	s.ddlView.ScrollToBeginning()
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
			SetTextColor(styles.Global.SecondaryTextColor.Color()).
			SetBackgroundColor(styles.Global.ContrastBackgroundColor.Color()).
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
			SetTextColor(styles.Global.SecondaryTextColor.Color()))
		s.table.SetCell(r+1, 1, tview.NewTableCell(" "+col.DataType+" ").
			SetTextColor(styles.Global.TextColor.Color()))
		s.table.SetCell(r+1, 2, tview.NewTableCell(" "+nullable+" ").
			SetTextColor(styles.Global.SecondaryTextColor.Color()))
		s.table.SetCell(r+1, 3, tview.NewTableCell(" "+def+" ").
			SetTextColor(styles.Global.TextColor.Color()))
		s.table.SetCell(r+1, 4, tview.NewTableCell(" "+strings.Join(constraints, ", ")+" ").
			SetTextColor(styles.Global.MoreContrastBackgroundColor.Color()))
	}

	if len(columns) > 0 {
		s.table.Select(1, 0)
	}
}
