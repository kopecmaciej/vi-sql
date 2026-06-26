package widget

import (
	"fmt"
	"math/rand"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

type Hints struct {
	*core.TextView
}

func NewHints() *Hints {
	h := &Hints{
		TextView: core.NewTextView(),
	}
	h.TextView.SetDynamicColors(true)
	h.TextView.SetBorderPadding(0, 0, 1, 1)

	return h
}

func (h *Hints) SetStyle(styles *config.Styles) {
	h.TextView.SetTextColor(styles.Global.SecondaryTextColor.Color())
	h.TextView.SetStyle(styles)
}

func (h *Hints) Render(keys *config.KeyBindings, nerdFont bool) {
	idx := rand.Intn(len(appHints))
	text := appHints[idx](keys)
	if nerdFont {
		h.TextView.SetText("💡 " + text)
	} else {
		h.TextView.SetText(" [::d]Hint:[-:-:-] " + text)
	}
}

var appHints = []func(k *config.KeyBindings) string{
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to open the actions panel — the fastest way to reach any major operation.", k.Main.OpenActions.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] on a selected row to peek at its full content in a side panel.", k.Data.PeekRow.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to hide the schema tree and give more space to the data view.", k.Main.HideSchema.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Use [::b]%s[-:-:-] to order by the column under the cursor without opening an order bar.", k.Data.OrderByColumn.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("[::b]%s[-:-:-] copies the current row to the clipboard in a key: value format.", k.Data.CopyRow.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("[::b]%s[-:-:-] opens the SQL query editor history so you can re-run or edit past queries.", k.SQLQueryEditor.OpenHistory.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to explain the current query and inspect its execution plan.", k.Data.ExplainQuery.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("[::b]%s[-:-:-] switches to a different style theme — try it to find one that suits your terminal.", k.Global.ChangeStyle.String())
	},
	func(k *config.KeyBindings) string {
		return "Use [::b]vi-sql --jump schema/table[-:-:-] to skip the connection page and land directly on a table."
	},
	func(k *config.KeyBindings) string {
		return "Enable the built-in MCP server in config to let AI assistants query your database through vi-sql."
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] in the SQL editor to open the query in your $EDITOR (enable it in options first).", k.SQLQueryEditor.TermEditor.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Use [::b]%s[-:-:-] to hide columns you don't need — press [::b]%s[-:-:-] to bring them all back.", k.Data.HideColumn.String(), k.Data.ResetHiddenColumns.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Use [::b]%s[-:-:-] to select multiple rows, then delete them in bulk.", k.Data.MultipleSelect.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("[::b]%s[-:-:-] exports the current result set to CSV, JSON, or SQL INSERT statements.", k.Data.ExportData.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Run [::b]vi-sql --debug[-:-:-] to enable verbose logging — check %s for details.", config.LogPath)
	},
	func(k *config.KeyBindings) string {
		return "Create your own style by adding a .yaml file to the styles folder in your config directory."
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to rename the active query tab — useful when working with multiple queries at once.", k.Main.RenameTab.String())
	},
	func(k *config.KeyBindings) string {
		return "Tabs with a table icon open live data with full CRUD — tabs with a terminal icon are free-form SQL editors."
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to open a blank query tab alongside any open table tab.", k.Main.NewTab.String())
	},
}
