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

func (h *Hints) Render(keys *config.KeyBindings, betterSymbols bool) {
	idx := rand.Intn(len(appHints))
	text := appHints[idx](keys)
	if betterSymbols {
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
		return fmt.Sprintf("[::b]%s[-:-:-] opens a new query tab so you can run SQL while keeping the current table visible.", k.Main.NewTab.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to hide the schema tree and give more space to the data view.", k.Main.HideSchema.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Use [::b]%s[-:-:-] to sort by the column under the cursor without opening a sort bar.", k.Data.SortByColumn.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("[::b]%s[-:-:-] copies the current row to the clipboard in a key: value format.", k.Data.CopyRow.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] on any key in this page to rebind it to your own preferred combination.", k.Common.Edit.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Use [::b]%s[-:-:-] / [::b]%s[-:-:-] to page through large result sets.", k.Data.PreviousPage.String(), k.Data.NextPage.String())
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
		return fmt.Sprintf("[::b]%s[-:-:-] duplicates the selected row — handy for inserting similar records quickly.", k.Data.DuplicateRow.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Use [::b]%s[-:-:-] to select multiple rows, then delete them in bulk.", k.Data.MultipleSelect.String())
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("[::b]%s[-:-:-] exports the current result set to CSV, JSON, or SQL INSERT statements.", k.Data.ExportData.String())
	},
	func(k *config.KeyBindings) string {
		return "Run [::b]vi-sql --debug[-:-:-] to enable verbose logging — check /tmp/vi-sql.log for details."
	},
	func(k *config.KeyBindings) string {
		return "Create your own style by adding a .yaml file to the styles folder in your config directory."
	},
	func(k *config.KeyBindings) string {
		return fmt.Sprintf("Press [::b]%s[-:-:-] to rename the active query tab — useful when working with multiple queries at once.", k.Main.RenameTab.String())
	},
}
