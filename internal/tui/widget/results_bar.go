package widget

import (
	"fmt"
	"strings"
	"time"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
)

// ResultsBar is a 1-2 line read-only text view that displays query result
// metadata: schema.table, row count, page, limit, execution time, and active
// WHERE/ORDER BY clauses.
type ResultsBar struct {
	*tview.TextView
	styles *config.Styles
}

func NewResultsBar() *ResultsBar {
	r := &ResultsBar{TextView: tview.NewTextView()}
	r.SetDynamicColors(true)
	return r
}

func (r *ResultsBar) SetStyle(styles *config.Styles) {
	r.styles = styles
	r.SetTextColor(styles.Content.StatusTextColor.Color())
}

// Render updates the bar text from the given table state and query metadata.
// When state.RawSQL is set the bar shows "sql" as the label and omits
// WHERE/ORDER BY (those are baked into the raw query).
func (r *ResultsBar) Render(state *database.TableState, execTime time.Duration, countPending bool) {
	if r.styles == nil {
		return
	}
	r.SetText(r.build(state, execTime, countPending))
}

// RenderStatementResult displays the result of a non-SELECT statement.
func (r *ResultsBar) RenderStatementResult(affected int64, execTime time.Duration) {
	if r.styles == nil {
		return
	}
	styles := r.styles
	textColor := styles.Global.TextColor.String()
	dimColor := "#64748B"
	sep := fmt.Sprintf(" [%s]│[-] ", dimColor)

	execColor := "#4ADE80"
	switch {
	case execTime >= 500*time.Millisecond:
		execColor = "#F87171"
	case execTime >= 100*time.Millisecond:
		execColor = styles.Global.SecondaryTextColor.String()
	}

	r.SetText(fmt.Sprintf("[%s]sql[-]%s[%s]%s rows affected[-]%s[%s]⏱ %s[-]",
		dimColor,
		sep,
		textColor, formatNumber(affected),
		sep,
		execColor, formatDuration(execTime),
	))
}

func (r *ResultsBar) build(state *database.TableState, execTime time.Duration, countPending bool) string {
	styles := r.styles
	textColor := styles.Global.TextColor.String()
	accentColor := styles.Global.SecondaryTextColor.String()
	blueColor := styles.Global.FocusColor.String()
	dimColor := "#64748B"
	sep := fmt.Sprintf(" [%s]│[-] ", dimColor)

	execColor := "#4ADE80"
	switch {
	case execTime >= 500*time.Millisecond:
		execColor = "#F87171"
	case execTime >= 100*time.Millisecond:
		execColor = accentColor
	}

	rowPrefix := ""
	if countPending {
		rowPrefix = "~"
	}

	label := fmt.Sprintf("[%s]%s.%s[-]", dimColor, state.Schema, state.Table)
	if state.RawSQL != "" {
		label = fmt.Sprintf("[%s]sql[-]", dimColor)
	}

	line1 := fmt.Sprintf("%s%s[%s]%s%s rows[-]%s[%s]pg %d/%d[-]%s[%s]limit %d[-]%s[%s]⏱ %s[-]",
		label,
		sep,
		textColor, rowPrefix, formatNumber(state.Count),
		sep,
		dimColor, state.GetCurrentPage(), state.GetTotalPages(),
		sep,
		dimColor, state.Limit,
		sep,
		execColor, formatDuration(execTime),
	)

	// WHERE / ORDER BY are baked into raw SQL — skip second line in SQL mode.
	if state.RawSQL != "" {
		return line1
	}

	whereVal := state.Where
	orderByVal := state.OrderBy

	if whereVal != "" || orderByVal != "" {
		_, _, width, _ := r.GetInnerRect()
		if width <= 0 {
			width = 80
		}
		// label overhead: "⚑ WHERE: " ≈ 10, "↕ ORDER BY: " ≈ 13, separator ≈ 3
		if whereVal != "" && orderByVal != "" {
			perVal := (width - 26) / 2
			whereVal = truncateStr(whereVal, perVal)
			orderByVal = truncateStr(orderByVal, perVal)
		} else if whereVal != "" {
			whereVal = truncateStr(whereVal, width-10)
		} else {
			orderByVal = truncateStr(orderByVal, width-13)
		}
	}

	var filters []string
	if whereVal != "" {
		filters = append(filters, fmt.Sprintf("[%s]⚑ WHERE:[-] [%s]%s[-]", accentColor, textColor, whereVal))
	}
	if orderByVal != "" {
		filters = append(filters, fmt.Sprintf("[%s]↕ ORDER BY:[-] [%s]%s[-]", blueColor, textColor, orderByVal))
	}

	if len(filters) == 0 {
		return line1
	}
	return line1 + "\n" + strings.Join(filters, sep)
}

func truncateStr(s string, max int) string {
	if max < 4 {
		max = 4
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-3]) + "..."
}

func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
