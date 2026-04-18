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
)

const (
	WelcomePageId = "Welcome"
)

type Welcome struct {
	*core.BaseElement
	*core.Flex

	form   *core.Form
	footer *component.Footer

	onSubmit func()
}

func NewWelcome() *Welcome {
	w := &Welcome{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		form:        core.NewForm(),
		footer:      component.NewFooter(),
	}

	w.SetIdentifier(WelcomePageId)

	return w
}

func (w *Welcome) Init(app *core.App) error {
	w.App = app

	if err := w.footer.Init(app); err != nil {
		return err
	}
	w.setLayout()
	w.setStyle()
	w.form.ApplyFormNavKeys(app.GetKeys())
	w.handleEvents()

	return nil
}

func (w *Welcome) setLayout() {
	w.form.SetBorder(true)
	w.form.SetTitle(" Welcome to Vi-SQL ")
	w.form.SetTitleAlign(tview.AlignCenter)
	w.form.SetButtonsAlign(tview.AlignCenter)
	w.footer.SetCentered(true)

	w.form.AddButton("Save and Connect", func() {
		err := w.saveConfig()
		if err != nil {
			showError(w.App.Pages, "Error while saving config", err)
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
				showError(w.App.Pages, "Error while saving config", err)
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

func (w *Welcome) setStyle() {
	styles := w.App.GetStyles()
	w.Flex.SetStyle(styles)
	w.form.SetStyle(styles)

	w.form.SetFieldTextColor(styles.Global.TextColor.Color())
	w.form.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	w.form.SetLabelColor(styles.Global.SecondaryTextColor.Color())
}

func (w *Welcome) handleEvents() {
	go w.HandleEvents(WelcomePageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			w.setStyle()
			go w.App.QueueUpdateDraw(func() {
				w.Render()
			})
		}
	})
}

func (w *Welcome) Render() {
	w.Clear()
	w.SetDirection(tview.FlexRow)

	centerFlex := tview.NewFlex()
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)
	w.renderForm()
	centerFlex.AddItem(w.form, 0, 3, true)
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)

	w.AddItem(centerFlex, 0, 1, true)
	w.renderFooter()
	w.AddItem(w.footer, 2, 0, false)

	if page, _ := w.App.Pages.GetFrontPage(); page == WelcomePageId {
		w.App.SetFocus(w)
	}
}

func (w *Welcome) renderFooter() {
	k := w.App.GetKeys()
	w.footer.SetKeys([]config.Key{
		k.Navigation.FocusUp,
		k.Navigation.FocusDown,
		k.Common.Confirm,
	})
}

func (w *Welcome) SetOnSubmitFunc(onSubmit func()) {
	w.onSubmit = onSubmit
}

func (w *Welcome) renderForm() {
	w.form.Clear(false)

	cfg := w.App.GetConfig()

	configFile, err := cfg.GetCurrentConfigPath()
	if err != nil {
		showError(w.App.Pages, "Error while getting config path", err)
		return
	}

	welcomeText := "All configuration can be set in " + configFile + " file. You can also set it here."
	w.form.AddTextView("Welcome info", welcomeText, 0, 2, true, false)
	w.form.AddTextView(" ", "----------------------------------------------------------", 0, 1, true, false)
	editorOptions := []string{"Built-in editor", "$EDITOR / command"}
	editorIdx := 0
	if !cfg.Editor.UseBuiltin {
		editorIdx = 1
	}
	w.form.AddFormItem(tview.NewButtonGroup("Editor choice", editorOptions, editorIdx, nil))
	w.form.AddTextView("External editor", "Set command (vim, nano etc) or env ($ENV)", 0, 1, true, false)
	editorCmd, err := cfg.GetEditorCmd()
	if err != nil {
		editorCmd = ""
	}
	w.form.AddInputField("Set editor", editorCmd, 30, nil, nil)
	w.form.AddTextView("Logs", "Requires restart if changed", 0, 1, true, false)
	w.form.AddInputField("Log File", cfg.Log.Path, 30, nil, nil)
	logLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	w.form.AddFormItem(tview.NewButtonGroup("Log Level", logLevels, getLogLevelIndex(cfg.Log.Level, logLevels), nil))
	w.form.AddCheckbox("Nerd Font icons", cfg.Styles.BetterSymbols, nil)
	w.form.AddTextView("Show on start", "Set pages to show on every start", 60, 1, true, false)
	w.form.AddCheckbox("Connection page", cfg.ShowConnectionPage, nil)
	w.form.AddTextView("Welcome page", "This page can be shown anytime via the -w flag", 60, 1, true, false)
	w.form.AddTextView("Keybindings", fmt.Sprintf("Press: '%s' help page, %s to expand footer keys", w.App.GetKeys().Global.FullScreenHelp.String(), w.App.GetKeys().Global.ToggleFooter.String()), 60, 1, true, false)
	w.form.AddTextView("Motions", "Use basic vim motions or normal arrow keys to move around", 60, 2, true, false)
	w.form.AddTextView(" ", "----------------------------------------------------------", 0, 1, true, false)
	w.form.AddTextView("MCP Server", "Expose database tools to Claude Code (or any MCP client)", 60, 1, true, false)
	w.form.AddCheckbox("MCP enabled", cfg.MCP.Enabled, nil)
	mcpPort := fmt.Sprintf("%d", cfg.MCP.Port)
	if cfg.MCP.Port == 0 {
		mcpPort = "9741"
	}
	w.form.AddInputField("MCP port", mcpPort, 10, nil, nil)
	w.form.AddCheckbox("Allow writes", cfg.MCP.AllowWrite, nil)
	w.form.AddTextView("MCP URL", fmt.Sprintf("http://localhost:%s/mcp  (add to Claude Code via: claude mcp add --transport http vi-sql http://localhost:%s/mcp)", mcpPort, mcpPort), 60, 2, true, false)
	w.form.ApplyDropdownNavKeys(w.App.GetKeys())
}

func (w *Welcome) saveConfig() error {
	editorCmd := w.form.GetFormItemByLabel("Set editor").(*tview.InputField).GetText()
	logFile := w.form.GetFormItemByLabel("Log File").(*tview.InputField).GetText()
	_, logLevelIdx := w.form.GetFormItemByLabel("Log Level").(*tview.ButtonGroup).GetCurrentOption()
	logLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	logLevel := logLevels[logLevelIdx]
	_, editorChoiceIdx := w.form.GetFormItemByLabel("Editor choice").(*tview.ButtonGroup).GetCurrentOption()

	c := w.App.GetConfig()

	c.Editor.UseBuiltin = editorChoiceIdx == 0
	splitEditorCmd := strings.Split(editorCmd, "$")
	if len(splitEditorCmd) > 1 {
		c.Editor.Command = ""
		c.Editor.Env = splitEditorCmd[1]
	} else {
		c.Editor.Env = ""
		c.Editor.Command = editorCmd
	}
	c.Log.Path = logFile
	c.Log.Level = logLevel
	c.ShowConnectionPage = w.form.GetFormItemByLabel("Connection page").(*tview.Checkbox).IsChecked()

	betterSymbols := w.form.GetFormItemByLabel("Nerd Font icons").(*tview.Checkbox).IsChecked()
	if betterSymbols != c.Styles.BetterSymbols {
		c.Styles.BetterSymbols = betterSymbols
		_ = w.App.SetStyle(c.Styles.CurrentStyle)
	}

	mcpPort := 9741
	if _, err := fmt.Sscanf(w.form.GetFormItemByLabel("MCP port").(*tview.InputField).GetText(), "%d", &mcpPort); err != nil {
		mcpPort = 9741
	}
	c.MCP.Enabled = w.form.GetFormItemByLabel("MCP enabled").(*tview.Checkbox).IsChecked()
	c.MCP.Port = mcpPort
	c.MCP.AllowWrite = w.form.GetFormItemByLabel("Allow writes").(*tview.Checkbox).IsChecked()

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
