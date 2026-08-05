package component

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

type searchState struct {
	input    *core.InputField
	text     string
	active   bool
	debounce *time.Timer
}

func (s *searchState) init(c *Data) {
	s.input = core.NewInputField()
	s.input.SetLabel(" / ")
	s.input.SetBorder(true)
	s.input.SetChangedFunc(func(text string) {
		s.onChange(c, text)
	})
	s.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			s.accept(c)
			return
		}
		s.exit(c)
	})
}

func (s *searchState) enter(c *Data) {
	s.active = true
	s.text = ""
	s.input.SetText("")
	c.Render()
	c.App.SetFocusOnly(s.input)
}

func (s *searchState) exit(c *Data) {
	s.active = false
	s.text = ""
	s.input.SetText("")
	if s.debounce != nil {
		s.debounce.Stop()
		s.debounce = nil
	}
	c.Render()
	c.reRenderState()
	c.App.SetFocusOnly(c.resultGrid)
}

func (s *searchState) accept(c *Data) {
	s.active = false
	if s.debounce != nil {
		s.debounce.Stop()
		s.debounce = nil
	}
	c.Render()
	c.reRenderState()
	c.App.SetFocusOnly(c.resultGrid)

	rows := s.filtered(c)
	matches := c.resultGrid.FindMatches(s.text, rows, c.columns)
	if len(matches) > 0 {
		c.resultGrid.Select(matches[0][0], matches[0][1])
	}
}

func (s *searchState) onChange(c *Data, text string) {
	s.text = text
	c.reRenderState()
	c.resultGrid.SetSelectable(false, false)
	if s.debounce != nil {
		s.debounce.Stop()
	}
	s.debounce = time.AfterFunc(400*time.Millisecond, func() {
		c.App.QueueUpdateDraw(func() {
			if s.text != "" {
				c.resultGrid.SetSelectable(true, true)
			}
		})
	})
}

func (s *searchState) clear() {
	s.text = ""
	s.active = false
	if s.debounce != nil {
		s.debounce.Stop()
		s.debounce = nil
	}
}

func (s *searchState) filtered(c *Data) []database.Row {
	rows := c.state.GetAllRows()
	if s.text == "" || len(rows) == 0 {
		return rows
	}
	return filterRowsBySearch(rows, s.text, c.columns, c.resultGrid.VisibleColumns(rows[0], c.columns))
}

func (s *searchState) style(styles *config.Styles) {
	s.input.SetBackgroundColor(styles.Global.BackgroundColor.Color())
	s.input.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	s.input.SetFieldTextColor(styles.Global.TextColor.Color())
	s.input.SetLabelStyle(tcell.StyleDefault.
		Foreground(styles.Global.FocusColor.Color()).
		Background(styles.Global.BackgroundColor.Color()))
}

func filterRowsBySearch(rows []database.Row, text string, cols []database.ColumnInfo, visibleCols []string) []database.Row {
	lower := strings.ToLower(text)
	boolCols := buildBoolCols(cols)
	filtered := make([]database.Row, 0)
	for _, row := range rows {
		for _, colName := range visibleCols {
			if strings.Contains(strings.ToLower(cellSearchText(row[colName], boolCols[colName])), lower) {
				filtered = append(filtered, row)
				break
			}
		}
	}
	return filtered
}
