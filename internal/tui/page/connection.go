package page

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	ConnectionPageId = "Connection"
)

type Connection struct {
	*core.BaseElement
	*core.Flex

	table   *core.Table
	preview *core.Table
	footer  *component.Footer

	style *config.ConnectionStyle

	onSubmit func()
	onCancel func()
}

func NewConnection() *Connection {
	c := &Connection{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		table:       core.NewTable(),
		preview:     core.NewTable(),
		footer:      component.NewFooter(),
	}
	c.footer.SetCentered(true)
	c.SetIdentifier(ConnectionPageId)
	return c
}

func (c *Connection) Init(app *core.App) error {
	c.App = app

	if err := c.footer.Init(app); err != nil {
		return err
	}
	c.setLayout()
	c.setStyle()
	c.setKeybindings()
	c.handleEvents()
	return nil
}

func (c *Connection) handleEvents() {
	go c.HandleEvents(ConnectionPageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			c.setStyle()
			go c.App.QueueUpdateDraw(func() {
				c.Render()
			})
		}
	})
}

func (c *Connection) setLayout() {
	c.table.SetTitle(" Saved Connections ")
	c.table.SetBorder(true)
	c.table.SetBorderPadding(0, 0, 1, 1)

	c.preview.SetBorder(true)
	c.preview.SetBorderPadding(0, 0, 1, 1)
}

func (c *Connection) setStyle() {
	c.SetStyle(c.App.GetStyles())
	c.table.SetStyle(c.App.GetStyles())
	c.preview.SetStyle(c.App.GetStyles())
	c.style = &c.App.GetStyles().Connection

	c.table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(c.style.ListSelectedTextColor.Color()).
		Background(c.style.ListSelectedBackgroundColor.Color()))
}

func (c *Connection) setKeybindings() {
	k := c.App.GetKeys()
	c.table.SetSelectionChangedFunc(func(row, col int) {
		c.updatePreview(row)
	})
	c.table.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Common.Select, event):
			row, _ := c.table.GetSelection()
			if row >= c.table.GetRowCount()-1 {
				c.openDriverPicker()
			} else {
				c.setConnection()
			}
			return nil
		case k.Match(k.Common.Add, event):
			c.openDriverPicker()
			return nil
		case k.Match(k.Common.Delete, event):
			c.deleteCurrConnection()
			return nil
		case k.Match(k.Common.Edit, event):
			c.openEditForm()
			return nil
		case k.Match(k.Common.Close, event):
			if c.onCancel != nil {
				c.onCancel()
			}
			return nil
		}
		return event
	}))
}

func (c *Connection) openDriverPicker() {
	picker := NewDriverPicker()
	if err := picker.Init(c.App); err != nil {
		modal.ShowError(c.App.Pages, "Failed to init driver picker", err)
		return
	}
	picker.SetOnSelectFunc(func(driver string) {
		c.App.Pages.RemoveModalPage(DriverPickerPageId)
		c.openAddFormWithDriver(driver)
	})
	picker.SetOnCancelFunc(func() {
		c.App.Pages.RemoveModalPage(DriverPickerPageId)
		c.App.SetFocus(c.table)
	})
	picker.Render()
	c.App.Pages.ShowModal(DriverPickerPageId, picker, picker, true, true)
}

func (c *Connection) openAddFormWithDriver(driver string) {
	form := NewConnectionForm(nil, driver)
	if err := form.Init(c.App); err != nil {
		modal.ShowError(c.App.Pages, "Failed to init connection form", err)
		return
	}
	form.SetOnSaveFunc(func() {
		c.App.Pages.RemoveModalPage(ConnectionFormPageId)
		c.Render()
		c.table.Select(c.table.GetRowCount()-2, 0)
		c.App.SetFocus(c.table)
	})
	form.SetOnCancelFunc(func() {
		c.App.Pages.RemoveModalPage(ConnectionFormPageId)
		c.App.SetFocus(c.table)
	})
	form.SetOnBackFunc(func() {
		c.App.Pages.RemoveModalPage(ConnectionFormPageId)
		c.openDriverPicker()
	})
	form.Render()
	c.App.Pages.ShowModal(ConnectionFormPageId, form, form.form, true, true)
}

func (c *Connection) openEditForm() {
	row, _ := c.table.GetSelection()
	if row == 0 || row >= c.table.GetRowCount()-1 {
		return
	}
	ref := c.table.GetCell(row, 1).GetReference()
	if ref == nil {
		return
	}
	connName := ref.(string)
	conn, err := c.App.GetConfig().GetConnectionByName(connName)
	if err != nil {
		modal.ShowError(c.App.Pages, "Failed to load connection for editing", err)
		return
	}

	form := NewConnectionForm(conn, "")
	if err := form.Init(c.App); err != nil {
		modal.ShowError(c.App.Pages, "Failed to init connection form", err)
		return
	}
	form.SetOnSaveFunc(func() {
		c.App.Pages.RemoveModalPage(ConnectionFormPageId)
		c.Render()
		c.table.Select(row, 0)
		c.App.SetFocus(c.table)
	})
	form.SetOnCancelFunc(func() {
		c.App.Pages.RemoveModalPage(ConnectionFormPageId)
		c.App.SetFocus(c.table)
	})
	form.Render()
	c.App.Pages.ShowModal(ConnectionFormPageId, form, form.form, true, true)
}

func (c *Connection) Render() {
	c.Clear()
	c.SetDirection(tview.FlexRow)

	c.renderTable()
	row, _ := c.table.GetSelection()
	c.updatePreview(row)

	sw, _ := c.App.ScreenSize()

	width := c.computeContentWidth()
	// Never let the content overflow the terminal width so the page stays
	// horizontally centered regardless of how wide the saved connections are.
	if sw > 0 && width > sw {
		width = sw
	}
	wrap := func(inner tview.Primitive, focus bool) *core.Flex {
		row := core.NewFlex()
		row.SetDirection(tview.FlexColumn)
		row.AddItem(tview.NewBox(), 0, 1, false)
		row.AddItem(inner, width, 0, focus)
		row.AddItem(tview.NewBox(), 0, 1, false)
		return row
	}

	c.AddItem(wrap(c.table, true), 0, 1, true)
	c.AddItem(wrap(c.preview, false), 7, 0, false)

	c.renderFooter()
	c.AddItem(c.footer, 2, 0, false)

	if page, _ := c.App.Pages.GetFrontPage(); page == ConnectionPageId {
		if c.table.GetRowCount() > 2 {
			defer c.App.SetFocusOnly(c.table)
		} else {
			defer c.openDriverPicker()
		}
	}
}

// sortedConnections returns a copy of connections sorted by LastUsed descending
// (most recently used first; never-used entries go to the bottom).
func sortedConnections(conns []config.SQLConfig) []config.SQLConfig {
	sorted := make([]config.SQLConfig, len(conns))
	copy(sorted, conns)
	sort.SliceStable(sorted, func(i, j int) bool {
		ti, tj := sorted[i].LastUsed, sorted[j].LastUsed
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.After(tj)
	})
	return sorted
}

func (c *Connection) computeContentWidth() int {
	runes := utf8.RuneCountInString
	max := func(a, b int) int {
		if a > b {
			return a
		}
		return b
	}

	headers := []string{"#", "Name", "Host:Port", "Database", "Driver", "Last Used"}
	const hostPortCap = 32
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = runes(h) + 2
	}

	conns := sortedConnections(c.App.GetConfig().Connections)
	for i, conn := range conns {
		hostPort := "—"
		if conn.Host != "" {
			hostPort = fmt.Sprintf("%s:%d", conn.Host, conn.Port)
		}
		if w := runes(hostPort); w > hostPortCap {
			hostPort = hostPort[:hostPortCap]
		}
		database := conn.Database
		if database == "" {
			database = "—"
		}
		lastUsed := "—"
		if !conn.LastUsed.IsZero() {
			lastUsed = conn.LastUsed.Format("2006-01-02 15:04")
		}

		cells := []string{
			fmt.Sprintf("%d", i+1),
			conn.Name,
			hostPort,
			database,
			conn.GetDriver(),
			lastUsed,
		}
		for j, cell := range cells {
			colWidths[j] = max(colWidths[j], runes(cell)+2)
		}
	}

	tableContent := 0
	for _, w := range colWidths {
		tableContent += w
	}
	tableWidth := tableContent + 4 + len(headers) // border (2) + inner padding (2) + inter-column spacing

	previewLabelMax := runes(" Timeout ")
	previewValueMax := runes("— ")
	for _, conn := range conns {
		dsn := conn.GetSafeDSN()
		if dsn == "" {
			dsn = "—"
		}
		username := conn.Username
		if username == "" {
			username = "—"
		}
		ssl := conn.SSLMode
		if ssl == "" {
			ssl = "—"
		}
		timeout := "default"
		if conn.Timeout > 0 {
			timeout = fmt.Sprintf("%ds", conn.Timeout)
		}
		for _, v := range []string{dsn, username, ssl, timeout} {
			previewValueMax = max(previewValueMax, runes(v)+1)
		}
	}
	previewWidth := previewLabelMax + previewValueMax + 4

	return max(tableWidth, previewWidth)
}

func (c *Connection) renderFooter() {
	k := c.App.GetKeys()
	keys := []config.Key{
		k.Common.Select,
		k.Common.Add,
		k.Common.Edit,
		k.Common.Delete,
		k.Global.FullScreenHelp,
	}
	if c.onCancel != nil {
		keys = append(keys, k.Common.Close)
	}
	c.footer.SetKeys(keys)
}

func (c *Connection) renderTable() {
	styles := c.App.GetStyles()
	contrastBg := c.style.TableHeaderBackgroundColor.Color()
	headerFg := c.style.TableHeaderTextColor.Color()
	textColor := styles.Global.TextColor.Color()
	secondaryColor := styles.Global.SecondaryTextColor.Color()
	dimColor := styles.Global.DimColor.Color()

	c.table.Clear()
	c.table.SetFixed(1, 0)
	c.table.SetSelectable(true, false)

	bg := styles.Global.BackgroundColor.Color()
	headerStyle := tcell.StyleDefault.Foreground(headerFg).Background(contrastBg)

	headers := []string{"#", "Name", "Host:Port", "Database", "Driver", "Last Used"}
	for i, h := range headers {
		c.table.SetCell(0, i, tview.NewTableCell(" "+h+" ").
			SetSelectable(false).
			SetStyle(headerStyle).
			SetAlign(tview.AlignCenter))
	}

	for i, conn := range sortedConnections(c.App.GetConfig().Connections) {
		hostPort := "—"
		if conn.Host != "" {
			hostPort = fmt.Sprintf("%s:%d", conn.Host, conn.Port)
		}
		database := conn.Database
		if database == "" {
			database = "—"
		}
		lastUsed := "—"
		if !conn.LastUsed.IsZero() {
			lastUsed = conn.LastUsed.Format("2006-01-02 15:04")
		}

		row := i + 1
		c.table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf(" %d ", i+1)).
			SetStyle(tcell.StyleDefault.Foreground(dimColor).Background(bg)).
			SetAlign(tview.AlignRight))
		c.table.SetCell(row, 1, tview.NewTableCell(" "+conn.Name+" ").
			SetReference(conn.Name).
			SetStyle(tcell.StyleDefault.Foreground(textColor).Background(bg)))
		c.table.SetCell(row, 2, tview.NewTableCell(" "+hostPort+" ").
			SetStyle(tcell.StyleDefault.Foreground(secondaryColor).Background(bg)).
			SetMaxWidth(32))
		c.table.SetCell(row, 3, tview.NewTableCell(" "+database+" ").
			SetStyle(tcell.StyleDefault.Foreground(secondaryColor).Background(bg)))
		c.table.SetCell(row, 4, tview.NewTableCell(" "+conn.GetDriver()+" ").
			SetStyle(tcell.StyleDefault.Foreground(textColor).Background(bg)))
		c.table.SetCell(row, 5, tview.NewTableCell(" "+lastUsed+" ").
			SetStyle(tcell.StyleDefault.Foreground(dimColor).Background(bg)))
	}

	newRow := len(c.App.GetConfig().Connections) + 1
	c.table.SetCell(newRow, 0, tview.NewTableCell(""))
	c.table.SetCell(newRow, 1, tview.NewTableCell(" > Add").
		SetStyle(tcell.StyleDefault.Foreground(c.style.ListTextColor.Color()).Background(bg)))
	for col := 2; col < len(headers); col++ {
		c.table.SetCell(newRow, col, tview.NewTableCell(""))
	}

	if len(c.App.GetConfig().Connections) > 0 {
		c.table.Select(1, 0)
	} else {
		c.table.Select(newRow, 0)
	}
}

func (c *Connection) setConnection() {
	row, _ := c.table.GetSelection()
	if row == 0 || row >= c.table.GetRowCount()-1 {
		return
	}
	ref := c.table.GetCell(row, 1).GetReference()
	if ref == nil {
		return
	}
	connName := ref.(string)
	err := c.App.GetConfig().SetCurrentConnection(connName)
	if err != nil {
		modal.ShowError(c.App.Pages, "Failed to set current connection", err)
		return
	}
	c.App.GetConfig().CurrentConnection = connName
	if c.onSubmit != nil {
		c.onSubmit()
	}
}

func (c *Connection) deleteCurrConnection() {
	row, _ := c.table.GetSelection()
	if row == 0 || row >= c.table.GetRowCount()-1 {
		return
	}
	ref := c.table.GetCell(row, 1).GetReference()
	if ref == nil {
		return
	}
	connName := ref.(string)
	conn, _ := c.App.GetConfig().GetConnectionByName(connName)
	err := c.App.GetConfig().DeleteConnection(connName)
	if err != nil {
		modal.ShowError(c.App.Pages, "Failed to delete connection", err)
		return
	}
	if conn != nil && conn.ID != "" {
		_ = modal.PurgeConnectionHistory(conn.ID)
	}

	c.Render()
	if row > 1 {
		c.table.Select(row-1, 0)
	}
}

func (c *Connection) SetOnSubmitFunc(onSubmit func()) {
	c.onSubmit = onSubmit
}

func (c *Connection) SetOnCancelFunc(onCancel func()) {
	c.onCancel = onCancel
}

func (c *Connection) updatePreview(row int) {
	if row == 0 || row >= c.table.GetRowCount()-1 {
		c.preview.SetTitle("")
		c.preview.Clear()
		return
	}
	ref := c.table.GetCell(row, 1).GetReference()
	if ref == nil {
		c.preview.SetTitle("")
		c.preview.Clear()
		return
	}
	// Preview uses the raw stored connection (not the decrypted variant from
	// GetConnectionByName) so the encryption-state check below reads the value
	// as it actually sits on disk.
	var conn *config.SQLConfig
	for i := range c.App.GetConfig().Connections {
		if c.App.GetConfig().Connections[i].Name == ref.(string) {
			conn = &c.App.GetConfig().Connections[i]
			break
		}
	}
	if conn == nil {
		c.preview.SetTitle("")
		c.preview.Clear()
		return
	}

	styles := c.App.GetStyles()
	dimColor := styles.Global.DimColor.Color()
	textColor := styles.Global.TextColor.Color()

	ssl := conn.SSLMode
	if ssl == "" {
		ssl = "—"
	}
	timeout := "default"
	if conn.Timeout > 0 {
		timeout = fmt.Sprintf("%ds", conn.Timeout)
	}
	username := conn.Username
	if username == "" {
		username = "—"
	}

	bg := styles.Global.BackgroundColor.Color()
	label := func(s string) *tview.TableCell {
		return tview.NewTableCell(" " + s + " ").SetStyle(tcell.StyleDefault.Foreground(dimColor).Background(bg))
	}
	value := func(s string) *tview.TableCell {
		return tview.NewTableCell(s + " ").
			SetStyle(tcell.StyleDefault.Foreground(textColor).Background(bg)).
			SetExpansion(1)
	}

	dsn := conn.GetSafeDSN()
	if dsn == "" {
		dsn = "—"
	}

	c.preview.Clear()
	c.preview.SetTitle(fmt.Sprintf(" %s ", conn.Name))

	c.preview.SetCell(0, 0, label("DSN"))
	c.preview.SetCell(0, 1, value(dsn))
	c.preview.SetCell(1, 0, label("User"))
	c.preview.SetCell(1, 1, value(username))
	c.preview.SetCell(2, 0, label("SSL"))
	c.preview.SetCell(2, 1, value(ssl))
	c.preview.SetCell(3, 0, label("Timeout"))
	c.preview.SetCell(3, 1, value(timeout))

	errorColor := styles.Global.ErrorColor.Color()
	warningColor := styles.Global.WarningColor.Color()
	switch {
	case conn.Password != "" && !util.IsEncrypted(conn.Password):
		c.preview.SetCell(4, 0, tview.NewTableCell(" ⚠ ").
			SetStyle(tcell.StyleDefault.Foreground(errorColor).Background(bg)))
		c.preview.SetCell(4, 1, tview.NewTableCell(fmt.Sprintf("[%s]password stored unencrypted[-]", styles.Global.ErrorColor)).SetExpansion(1))
	case !conn.IsPasswordReadable():
		storedMethod := util.ParseMethodTag(conn.Password)
		currentMethod := c.App.GetConfig().Security.Method
		var msg string
		if storedMethod != "" && storedMethod != currentMethod {
			msg = fmt.Sprintf("[%s]password encrypted with %s (current method: %s) — re-enter to re-encrypt[-]", styles.Global.WarningColor, storedMethod, currentMethod)
		} else {
			msg = fmt.Sprintf("[%s]password unreadable — key mismatch, re-enter[-]", styles.Global.WarningColor)
		}
		c.preview.SetCell(4, 0, tview.NewTableCell(" ⚠ ").
			SetStyle(tcell.StyleDefault.Foreground(warningColor).Background(bg)))
		c.preview.SetCell(4, 1, tview.NewTableCell(msg).SetExpansion(1))
	}
}
