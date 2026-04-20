package page

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
)

const (
	OptionsPageId = "Options"
)

// itemDescriptions maps focusable form item labels to help text shown in the side panel.
var itemDescriptions = map[string]string{
	"Enable $EDITOR":  "Open SQL queries in an external editor (vim, nano, etc.) instead of the built-in editor.\n\nOnce enabled, use the 'Move to $EDITOR' keybinding from within the SQL editor to transfer the current query.\n\n[::b]Note: the built-in editor has SQL autocomplete; external editors do not.[::-]",
	"Set editor":      "Command to invoke as the external editor.\n\nUse a bare command (e.g. 'vim') or prefix with '$' to read from an env var (e.g. '$EDITOR').",
	"Log File":        "Path where structured log output is written.\n\nDefault: /tmp/vi-sql.log\n\nChange takes effect on restart.",
	"Log Level":       "Controls how verbose the log output is.\n\n'debug' logs everything; 'info' is suitable for normal use; 'error' logs only failures.\n\nChange takes effect on restart.",
	"Nerd Font icons": "Enable Nerd Font symbols for richer icons in the schema tree and UI.\n\nRequires a Nerd Font to be installed and selected in your terminal emulator (e.g. JetBrainsMono Nerd Font).",
	"Connection page": "Show the connection selection page on every startup.\n\nWhen disabled, vi-sql connects to the last-used connection automatically.",
	"MCP enabled":     "Start an HTTP MCP server when vi-sql launches.\n\nAdd to Claude Code with:\n  claude mcp add --transport http vi-sql http://localhost:<port>/mcp",
	"MCP port":        "TCP port the MCP server listens on.\n\nDefault: 9741. Change this if the port is already in use.",
	"Allow execute":   "Allow the MCP client to execute SQL queries directly against the database.\n\nWhen disabled, the AI can only open queries in a vi-sql tab for you to review and run manually.\n\n[::b]Recommended: leave off unless you trust the AI to query without confirmation.[::-]",
	"Allow writes":    "Allow the MCP client to execute INSERT, UPDATE, DELETE, and DDL statements.\n\nOnly takes effect when 'Allow execute' is also enabled.\n\nDisabled by default. Enable only when you fully trust the AI agent and want it to modify data.",
}

type Options struct {
	*core.BaseElement
	*core.Flex

	form          *core.Form
	footer        *component.Footer
	descPanel     *tview.TextView
	mcpEnabled    bool
	mcpOptions    *core.FormGroup
	editorEnabled bool
	editorOptions *core.FormGroup
	groups        []*core.FormGroup

	onSubmit func()
}

func NewOptions() *Options {
	w := &Options{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		form:        core.NewForm(),
		footer:      component.NewFooter(),
		descPanel:   tview.NewTextView(),
	}

	w.SetIdentifier(OptionsPageId)

	return w
}

func (w *Options) Init(app *core.App) error {
	w.App = app

	if err := w.footer.Init(app); err != nil {
		return err
	}
	w.setLayout()
	w.setStyle()
	w.form.ApplyFormNavKeys(app.GetKeys())

	// Wrap to update description panel after every keypress (focus may have moved).
	prev := w.form.GetInputCapture()
	w.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		go w.App.QueueUpdateDraw(w.updateDescription)
		if prev != nil {
			return prev(event)
		}
		return event
	})

	w.handleEvents()

	return nil
}

func (w *Options) setLayout() {
	w.form.SetBorder(true)
	w.form.SetTitle(" Options ")
	w.form.SetTitleAlign(tview.AlignCenter)
	w.form.SetButtonsAlign(tview.AlignCenter)
	w.footer.SetCentered(true)

	w.descPanel.SetBorder(true)
	w.descPanel.SetTitle(" Info ")
	w.descPanel.SetWordWrap(true)
	w.descPanel.SetDynamicColors(true)
	w.descPanel.SetScrollable(false)

	w.form.AddButton("Save", func() {
		err := w.saveConfig()
		if err != nil {
			modal.ShowError(w.App.Pages, "Error while saving config", err)
			return
		}
		if w.onSubmit != nil {
			w.onSubmit()
		}
	})

	w.form.AddButton("Exit", func() {
		w.App.Stop()
	})

	w.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		k := w.App.GetKeys()
		if k.Contains(k.Common.Confirm, event.Name()) {
			err := w.saveConfig()
			if err != nil {
				modal.ShowError(w.App.Pages, "Error while saving config", err)
				return nil
			}
			if w.onSubmit != nil {
				w.onSubmit()
			}
			return nil
		}
		return event
	})
}

func (w *Options) setStyle() {
	styles := w.App.GetStyles()
	w.Flex.SetStyle(styles)
	w.form.SetStyle(styles)

	w.form.SetFieldTextColor(styles.Global.TextColor.Color())
	w.form.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	w.form.SetLabelColor(styles.Global.SecondaryTextColor.Color())

	w.descPanel.SetBackgroundColor(styles.Global.BackgroundColor.Color())
	w.descPanel.SetBorderColor(styles.Global.BorderColor.Color())
	w.descPanel.SetTitleColor(styles.Global.TitleColor.Color())
	w.descPanel.SetTextColor(styles.Global.SecondaryTextColor.Color())
}

func (w *Options) handleEvents() {
	go w.HandleEvents(OptionsPageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			w.setStyle()
			go w.App.QueueUpdateDraw(func() {
				w.Render()
			})
		}
	})
}

func (w *Options) updateDescription() {
	formIdx, _ := w.form.GetFocusedItemIndex()
	var text string
	if formIdx >= 0 && formIdx < w.form.GetFormItemCount() {
		label := strings.TrimSpace(w.form.GetFormItem(formIdx).GetLabel())
		text = itemDescriptions[label]
	}
	if text == "" {
		k := w.App.GetKeys()
		text = fmt.Sprintf("Navigate with %s / %s.\n\nPress 'Save' to apply settings.",
			k.Navigation.FocusUp.String(), k.Navigation.FocusDown.String())
	}
	w.descPanel.SetText(text)
}

func (w *Options) Render() {
	w.Clear()
	w.SetDirection(tview.FlexRow)

	centerFlex := tview.NewFlex()
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)
	w.renderForm()
	centerFlex.AddItem(w.form, 0, 4, true)
	centerFlex.AddItem(w.descPanel, 0, 2, false)

	w.AddItem(centerFlex, 0, 1, true)
	w.renderFooter()
	w.AddItem(w.footer, 2, 0, false)

	w.updateDescription()

	if page, _ := w.App.Pages.GetFrontPage(); page == OptionsPageId {
		w.App.SetFocusOnly(w)
	}
}

func (w *Options) renderFooter() {
	k := w.App.GetKeys()
	w.footer.SetKeys([]config.Key{
		k.Navigation.FocusUp,
		k.Navigation.FocusDown,
		k.Common.Confirm,
	})
}

func (w *Options) SetOnSubmitFunc(onSubmit func()) {
	w.onSubmit = onSubmit
}

func (w *Options) buildGroups() {
	if w.groups != nil {
		return
	}
	cfg := w.App.GetConfig()
	w.mcpEnabled = cfg.MCP.Enabled

	configFile, _ := cfg.GetCurrentConfigPath()

	w.mcpOptions = core.NewFormGroup(w.mcpEnabled, func() []tview.FormItem {
		mcpPort := fmt.Sprintf("%d", cfg.MCP.Port)
		if cfg.MCP.Port == 0 {
			mcpPort = "9741"
		}
		mcpURL := fmt.Sprintf("http://localhost:%s/mcp", mcpPort)
		return []tview.FormItem{
			tview.NewInputField().SetLabel("MCP port").SetText(mcpPort).SetFieldWidth(10),
			tview.NewCheckbox().SetLabel("Allow execute").SetChecked(cfg.MCP.AllowExecute),
			tview.NewCheckbox().SetLabel("Allow writes").SetChecked(cfg.MCP.AllowWrite),
			tview.NewTextView().SetLabel("MCP URL").
				SetText(mcpURL).
				SetSize(1, 60).SetDynamicColors(true).SetScrollable(false),
		}
	})

	w.editorEnabled = cfg.Editor.Enabled
	editorCmd, _ := cfg.GetEditorCmd()

	w.editorOptions = core.NewFormGroup(w.editorEnabled, func() []tview.FormItem {
		return []tview.FormItem{
			tview.NewInputField().SetLabel("Set editor").SetText(editorCmd).SetFieldWidth(30),
		}
	})

	w.groups = []*core.FormGroup{
		core.NewFormGroup(true, func() []tview.FormItem {
			return []tview.FormItem{
				tview.NewTextView().SetLabel("Config file").
					SetText(configFile).
					SetSize(1, 0).SetDynamicColors(true).SetScrollable(false),
			}
		}),
		core.NewFormGroup(true, func() []tview.FormItem {
			logLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
			return []tview.FormItem{
				tview.NewInputField().SetLabel("Log File").SetText(cfg.Log.Path).SetFieldWidth(30),
				tview.NewButtonGroup("Log Level", logLevels, getLogLevelIndex(cfg.Log.Level, logLevels), nil),
				tview.NewCheckbox().SetLabel("Nerd Font icons").SetChecked(cfg.Styles.BetterSymbols),
				tview.NewCheckbox().SetLabel("Connection page").SetChecked(cfg.ShowConnectionPage),
			}
		}),
		core.NewFormGroup(true, func() []tview.FormItem {
			return []tview.FormItem{
				tview.NewCheckbox().SetLabel("Enable $EDITOR").SetChecked(w.editorEnabled).
					SetChangedFunc(func(checked bool) {
						w.editorEnabled = checked
						w.editorOptions.SetVisible(checked)
						w.form.RenderGroups(w.groups)
						w.form.ApplyDropdownNavKeys(w.App.GetKeys())
						if idx := w.form.GetFormItemIndex("Enable $EDITOR"); idx >= 0 {
							w.form.SetFocus(idx)
						}
						w.App.SetFocusOnly(w.form)
					}),
			}
		}),
		w.editorOptions,
		core.NewFormGroup(true, func() []tview.FormItem {
			return []tview.FormItem{
				tview.NewCheckbox().SetLabel("MCP enabled").SetChecked(w.mcpEnabled).
					SetChangedFunc(func(checked bool) {
						w.mcpEnabled = checked
						w.mcpOptions.SetVisible(checked)
						w.form.RenderGroups(w.groups)
						w.form.ApplyDropdownNavKeys(w.App.GetKeys())
						if idx := w.form.GetFormItemIndex("MCP enabled"); idx >= 0 {
							w.form.SetFocus(idx)
						}
						w.App.SetFocusOnly(w.form)
					}),
			}
		}),
		w.mcpOptions,
	}
}

func (w *Options) renderForm() {
	w.buildGroups()
	w.form.RenderGroups(w.groups)
	w.form.ApplyDropdownNavKeys(w.App.GetKeys())
}

func (w *Options) saveConfig() error {
	logFile := w.form.GetFormItemByLabel("Log File").(*tview.InputField).GetText()
	_, logLevelIdx := w.form.GetFormItemByLabel("Log Level").(*tview.ButtonGroup).GetCurrentOption()
	logLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	logLevel := logLevels[logLevelIdx]

	c := w.App.GetConfig()

	c.Editor.Enabled = w.form.GetFormItemByLabel("Enable $EDITOR").(*tview.Checkbox).IsChecked()
	if w.editorOptions != nil && w.editorOptions.IsVisible() {
		editorCmd := w.form.GetFormItemByLabel("Set editor").(*tview.InputField).GetText()
		splitEditorCmd := strings.Split(editorCmd, "$")
		if len(splitEditorCmd) > 1 {
			c.Editor.Command = ""
			c.Editor.Env = splitEditorCmd[1]
		} else {
			c.Editor.Env = ""
			c.Editor.Command = editorCmd
		}
	}
	c.Log.Path = logFile
	c.Log.Level = logLevel
	c.ShowConnectionPage = w.form.GetFormItemByLabel("Connection page").(*tview.Checkbox).IsChecked()

	betterSymbols := w.form.GetFormItemByLabel("Nerd Font icons").(*tview.Checkbox).IsChecked()
	if betterSymbols != c.Styles.BetterSymbols {
		c.Styles.BetterSymbols = betterSymbols
		_ = w.App.SetStyle(c.Styles.CurrentStyle)
	}

	c.MCP.Enabled = w.form.GetFormItemByLabel("MCP enabled").(*tview.Checkbox).IsChecked()
	if w.mcpOptions != nil && w.mcpOptions.IsVisible() {
		mcpPort := 9741
		if _, err := fmt.Sscanf(w.form.GetFormItemByLabel("MCP port").(*tview.InputField).GetText(), "%d", &mcpPort); err != nil {
			mcpPort = 9741
		}
		c.MCP.Port = mcpPort
		c.MCP.AllowExecute = w.form.GetFormItemByLabel("Allow execute").(*tview.Checkbox).IsChecked()
		c.MCP.AllowWrite = w.form.GetFormItemByLabel("Allow writes").(*tview.Checkbox).IsChecked()
	}

	return w.App.GetConfig().UpdateConfig()
}

func getLogLevelIndex(currentLevel string, levels []string) int {
	for i, level := range levels {
		if level == currentLevel {
			return i
		}
	}
	return 0
}
