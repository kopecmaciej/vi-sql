package primitives

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
)

// RowLine represents a single row field displayed in the ViewModal.
type RowLine struct {
	Key   string
	Type  string
	Value string
	IsPK  bool
}

// ViewModal is a centered modal that displays SQL row data in a vertical
// key / type / value format with scrollable, highlighted navigation.
// Long values can be expanded inline with Enter.
type ViewModal struct {
	*tview.Box

	frame *tview.Frame
	form  *tview.Form

	rows []RowLine

	// selectedRow is the absolute index into m.rows (0-based).
	selectedRow int
	// scrollPosition is the first visible data row index.
	scrollPosition int
	// expanded tracks which rows have their value expanded inline.
	expanded map[int]bool

	// Colors
	keyColor       tcell.Color
	typeColor      tcell.Color
	valueColor     tcell.Color
	highlightColor tcell.Color

	// Margins (top/bottom padding inside the modal)
	marginTop    int
	marginBottom int

	isFullScreen bool

	done func(buttonIndex int, buttonLabel string)
}

// NewViewModal returns a new ViewModal.
func NewViewModal() *ViewModal {
	m := &ViewModal{
		Box:          tview.NewBox(),
		marginTop:    6,
		marginBottom: 6,
		expanded:     make(map[int]bool),
		keyColor:     tview.Styles.SecondaryTextColor,
		typeColor:    tview.Styles.TertiaryTextColor,
		valueColor:   tview.Styles.PrimaryTextColor,
	}

	m.form = tview.NewForm().
		SetButtonsAlign(tview.AlignCenter).
		SetButtonBackgroundColor(tview.Styles.PrimitiveBackgroundColor).
		SetButtonTextColor(tview.Styles.PrimaryTextColor)
	m.form.SetBackgroundColor(tview.Styles.ContrastBackgroundColor).
		SetBorderPadding(0, 0, 0, 0)
	m.form.SetCancelFunc(func() {
		if m.done != nil {
			m.done(-1, "")
		}
	})

	m.frame = tview.NewFrame(m.form).SetBorders(0, 0, 1, 0, 0, 0)
	m.frame.SetBorder(true).
		SetBackgroundColor(tview.Styles.ContrastBackgroundColor).
		SetBorderPadding(1, 1, 1, 1)

	return m
}

// --- Style setters ---

func (m *ViewModal) SetBackgroundColor(color tcell.Color) *tview.Box {
	m.Box.SetBackgroundColor(color)
	m.form.SetBackgroundColor(color)
	m.frame.SetBackgroundColor(color)
	return m.Box
}

func (m *ViewModal) SetBorderColor(color tcell.Color) *tview.Box {
	m.Box.SetBorderColor(color)
	m.frame.SetBorderColor(color)
	return m.Box
}

func (m *ViewModal) SetTitleColor(color tcell.Color) *tview.Box {
	m.Box.SetTitleColor(color)
	m.frame.SetTitleColor(color)
	return m.Box
}

func (m *ViewModal) SetFocusStyle(style tcell.Style) *tview.Box {
	m.Box.SetFocusStyle(style)
	m.frame.SetFocusStyle(style)
	return m.Box
}

func (m *ViewModal) SetButtonBackgroundColor(color tcell.Color) *ViewModal {
	m.form.SetButtonBackgroundColor(color)
	return m
}

func (m *ViewModal) SetButtonTextColor(color tcell.Color) *ViewModal {
	m.form.SetButtonTextColor(color)
	return m
}

func (m *ViewModal) SetHighlightColor(color tcell.Color) *ViewModal {
	m.highlightColor = color
	return m
}

func (m *ViewModal) SetDocumentColors(keyColor, valueColor, typeColor tcell.Color) *ViewModal {
	m.keyColor = keyColor
	m.valueColor = valueColor
	m.typeColor = typeColor
	return m
}

func (m *ViewModal) SetButtonStyle(style tcell.Style) *ViewModal {
	m.form.SetButtonStyle(style)
	return m
}

func (m *ViewModal) SetButtonActivatedStyle(style tcell.Style) *ViewModal {
	m.form.SetButtonActivatedStyle(style)
	return m
}

// --- Done / buttons ---

func (m *ViewModal) SetDoneFunc(handler func(buttonIndex int, buttonLabel string)) *ViewModal {
	m.done = handler
	return m
}

func (m *ViewModal) AddButtons(labels []string) *ViewModal {
	for index, label := range labels {
		func(i int, l string) {
			m.form.AddButton(label, func() {
				if m.done != nil {
					m.done(i, l)
				}
			})
			button := m.form.GetButton(m.form.GetButtonCount() - 1)
			button.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Rune() {
				case 'h':
					return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
				case 'l':
					return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
				}
				return event
			})
		}(index, label)
	}
	return m
}

func (m *ViewModal) ClearButtons() *ViewModal {
	m.form.ClearButtons()
	return m
}

// --- Data ---

// SetRows sets the row lines to display and resets scroll/expansion state.
func (m *ViewModal) SetRows(rows []RowLine) {
	m.rows = rows
	m.scrollPosition = 0
	m.selectedRow = 0
	m.expanded = make(map[int]bool)
}

// --- Full screen ---

func (m *ViewModal) SetFullScreen(fullScreen bool) *ViewModal {
	m.isFullScreen = fullScreen
	return m
}

func (m *ViewModal) IsFullScreen() bool {
	return m.isFullScreen
}

// --- Focus ---

func (m *ViewModal) Focus(delegate func(p tview.Primitive)) {
	delegate(m.form)
}

func (m *ViewModal) HasFocus() bool {
	return m.form.HasFocus()
}

// --- Navigation ---

func (m *ViewModal) MoveUp() {
	if m.selectedRow > 0 {
		m.selectedRow--
	}
}

func (m *ViewModal) MoveDown() {
	if m.selectedRow < len(m.rows)-1 {
		m.selectedRow++
	}
}

func (m *ViewModal) MoveToTop() {
	m.scrollPosition = 0
	m.selectedRow = 0
}

func (m *ViewModal) MoveToBottom() {
	if len(m.rows) > 0 {
		m.selectedRow = len(m.rows) - 1
	}
}

// ToggleExpand expands or collapses the value of the currently selected row.
func (m *ViewModal) ToggleExpand() {
	if m.selectedRow < 0 || m.selectedRow >= len(m.rows) {
		return
	}
	m.expanded[m.selectedRow] = !m.expanded[m.selectedRow]
}

// --- Copy ---

// CopySelectedLine copies the selected row line.
// copyType "full" copies "key: value", "value" copies value only.
func (m *ViewModal) CopySelectedLine(copyFunc func(text string) error, copyType string) error {
	if m.selectedRow < 0 || m.selectedRow >= len(m.rows) {
		return nil
	}

	rl := m.rows[m.selectedRow]
	var text string
	switch copyType {
	case "full":
		text = fmt.Sprintf("%s: %s", rl.Key, rl.Value)
	case "value":
		text = rl.Value
	default:
		text = fmt.Sprintf("%s: %s", rl.Key, rl.Value)
	}
	return copyFunc(strings.TrimSpace(text))
}

// --- Helpers ---

const maxTypeCap = 12

// wrapText splits text into lines of at most width characters,
// preferring to break at spaces.
func wrapText(text string, width int) []string {
	if width <= 0 || len(text) <= width {
		return []string{text}
	}
	var lines []string
	for len(text) > 0 {
		if len(text) <= width {
			lines = append(lines, text)
			break
		}
		breakAt := width
		for i := width; i > width/2; i-- {
			if text[i] == ' ' || text[i] == ',' || text[i] == ';' {
				breakAt = i + 1
				break
			}
		}
		lines = append(lines, text[:breakAt])
		text = text[breakAt:]
	}
	return lines
}

// expandedValueWidth returns the width available for wrapped value lines.
func (m *ViewModal) expandedValueWidth(innerWidth int) int {
	w := innerWidth - 4 // 4-space indent
	if w < 20 {
		w = 20
	}
	return w
}

// rowVisualHeight returns how many visual lines a data row occupies.
func (m *ViewModal) rowVisualHeight(idx, expandedWidth int) int {
	if !m.expanded[idx] {
		return 1
	}
	return 1 + len(wrapText(m.rows[idx].Value, expandedWidth))
}

// adjustScroll ensures selectedRow is visible within the available visual lines.
func (m *ViewModal) adjustScroll(maxVisualLines, expandedWidth int) {
	totalRows := len(m.rows)
	if totalRows == 0 {
		return
	}

	// Clamp selectedRow
	if m.selectedRow < 0 {
		m.selectedRow = 0
	}
	if m.selectedRow >= totalRows {
		m.selectedRow = totalRows - 1
	}

	// If selected is above scroll, jump up
	if m.selectedRow < m.scrollPosition {
		m.scrollPosition = m.selectedRow
	}

	// If selected is below visible area, scroll down
	for {
		usedLines := 0
		for i := m.scrollPosition; i <= m.selectedRow && i < totalRows; i++ {
			usedLines += m.rowVisualHeight(i, expandedWidth)
		}
		if usedLines <= maxVisualLines || m.scrollPosition >= m.selectedRow {
			break
		}
		m.scrollPosition++
	}

	// Clamp scrollPosition
	if m.scrollPosition < 0 {
		m.scrollPosition = 0
	}
}

// --- Draw ---

func (m *ViewModal) Draw(screen tcell.Screen) {
	screenWidth, screenHeight := screen.Size()

	var width, x, y int
	if m.isFullScreen {
		width = screenWidth
		x, y = 0, 0
	} else {
		width = screenWidth * 3 / 4
		if width < 80 {
			width = screenWidth - 4
		}
		x = (screenWidth - width) / 2
		y = m.marginTop / 2
	}

	maxVisualLines := screenHeight - m.marginTop - m.marginBottom
	if m.isFullScreen {
		maxVisualLines = screenHeight - m.marginBottom
	}
	if maxVisualLines < 1 {
		maxVisualLines = 1
	}

	// Calculate column widths for alignment.
	maxKeyLen, maxTypeLen := 0, 0
	for _, rl := range m.rows {
		if len(rl.Key) > maxKeyLen {
			maxKeyLen = len(rl.Key)
		}
		if len(rl.Type) > maxTypeLen {
			maxTypeLen = len(rl.Type)
		}
	}
	if maxTypeLen > maxTypeCap {
		maxTypeLen = maxTypeCap
	}

	innerWidth := width - 8
	maxValueLen := innerWidth - maxKeyLen - maxTypeLen - 6
	if maxValueLen < 20 {
		maxValueLen = 20
	}
	expWidth := m.expandedValueWidth(innerWidth)

	// Adjust scroll so selectedRow is visible
	m.adjustScroll(maxVisualLines, expWidth)

	m.frame.Clear()
	totalRows := len(m.rows)
	visualLine := 0

	for rowIdx := m.scrollPosition; rowIdx < totalRows && visualLine < maxVisualLines; rowIdx++ {
		rl := m.rows[rowIdx]
		isSelected := rowIdx == m.selectedRow
		isExpanded := m.expanded[rowIdx]

		// Truncate type
		displayType := rl.Type
		if len(displayType) > maxTypeLen {
			displayType = displayType[:maxTypeLen-1] + "…"
		}

		needsExpansion := len(rl.Value) > maxValueLen

		if isExpanded && needsExpansion {
			// Header line: key + type + ▼ marker
			line := fmt.Sprintf("[%s]%-*s  [%s]%-*s  [%s]%s",
				m.keyColor.CSS(), maxKeyLen, rl.Key,
				m.typeColor.CSS(), maxTypeLen, displayType,
				m.valueColor.CSS(), "▼",
			)
			if isSelected {
				line = fmt.Sprintf("[-:%s:b]>%s[-:-:-]", m.highlightColor.CSS(), line)
			} else {
				line = " " + line
			}
			m.frame.AddText(line, true, tview.AlignLeft, tcell.ColorDefault)
			visualLine++

			// Wrapped value lines
			indent := "    "
			wrapped := wrapText(rl.Value, expWidth)
			for _, wl := range wrapped {
				if visualLine >= maxVisualLines {
					break
				}
				valueLine := fmt.Sprintf("[%s]%s%s", m.valueColor.CSS(), indent, wl)
				if isSelected {
					valueLine = fmt.Sprintf("[-:%s:b] %s[-:-:-]", m.highlightColor.CSS(), valueLine)
				} else {
					valueLine = " " + valueLine
				}
				m.frame.AddText(valueLine, true, tview.AlignLeft, tcell.ColorDefault)
				visualLine++
			}
		} else {
			// Single line: key + type + value (truncated if needed)
			displayValue := rl.Value
			if needsExpansion {
				displayValue = displayValue[:maxValueLen-3] + "..."
			}

			line := fmt.Sprintf("[%s]%-*s  [%s]%-*s  [%s]%s",
				m.keyColor.CSS(), maxKeyLen, rl.Key,
				m.typeColor.CSS(), maxTypeLen, displayType,
				m.valueColor.CSS(), displayValue,
			)

			if isSelected {
				line = fmt.Sprintf("[-:%s:b]>%s[-:-:-]", m.highlightColor.CSS(), line)
			} else {
				line = " " + line
			}
			m.frame.AddText(line, true, tview.AlignLeft, tcell.ColorDefault)
			visualLine++
		}
	}

	height := maxVisualLines + m.marginBottom
	m.SetRect(x, y, width, height)

	if m.isFullScreen {
		height = screenHeight
	}
	m.frame.SetRect(x, y, width, height)
	m.frame.Draw(screen)
}

// --- Input handling ---

func (m *ViewModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return m.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyDown:
			m.MoveDown()
			return
		case tcell.KeyUp:
			m.MoveUp()
			return
		case tcell.KeyEnter:
			m.ToggleExpand()
			return
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				m.MoveDown()
				return
			case 'k':
				m.MoveUp()
				return
			}
		}

		if m.frame.HasFocus() {
			if handler := m.frame.InputHandler(); handler != nil {
				handler(event, setFocus)
				return
			}
		}
	})
}

// --- Mouse handling ---

func (m *ViewModal) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return m.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		consumed, capture = m.form.MouseHandler()(action, event, setFocus)
		if !consumed && action == tview.MouseLeftDown && m.InRect(event.Position()) {
			setFocus(m)
			consumed = true
		}
		return
	})
}
