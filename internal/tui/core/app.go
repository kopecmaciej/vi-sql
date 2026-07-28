package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type App struct {
	*tview.Application

	Pages                *Pages
	driver               database.Driver
	formatter            database.ValueFormatter
	manager              *manager.ElementManager
	styles               *config.Styles
	config               *config.Config
	keys                 *config.KeyBindings
	focusStack           []tview.Primitive
	lastFocusedPrimitive tview.Primitive
	mcpEnabled           bool
	cursorStyle          tcell.CursorStyle
	openStyleModal       func()
	openConnectionPage   func()
	openOptionsPage      func()
	toggleMCP            func()
}

func (a *App) SetOpenStyleModalFunc(fn func()) { a.openStyleModal = fn }

func (a *App) OpenStyleModal() {
	if a.openStyleModal != nil {
		a.openStyleModal()
	}
}

func (a *App) SetOpenConnectionPageFunc(fn func()) { a.openConnectionPage = fn }

func (a *App) OpenConnectionPage() {
	if a.openConnectionPage != nil {
		a.openConnectionPage()
	}
}

func (a *App) SetOpenOptionsPageFunc(fn func()) { a.openOptionsPage = fn }

func (a *App) OpenOptionsPage() {
	if a.openOptionsPage != nil {
		a.openOptionsPage()
	}
}

func (a *App) SetToggleMCPFunc(fn func()) { a.toggleMCP = fn }

func (a *App) ToggleMCP() {
	if a.toggleMCP != nil {
		a.toggleMCP()
	}
}

func (a *App) SetMCPEnabled(enabled bool) { a.mcpEnabled = enabled }

func (a *App) IsMCPEnabled() bool { return a.mcpEnabled }

func NewApp(appConfig *config.Config) *App {
	styles, err := config.LoadStyles(appConfig.Styles.CurrentStyle, appConfig.Styles.NerdFont)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load styles")
	}
	styles.LoadMainStyles()
	keyBindings, err := config.LoadKeybindings(appConfig.UI.VimMode)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load keybindings")
	}

	app := &App{
		Application: tview.NewApplication(),
		manager:     manager.NewElementManager(),
		styles:      styles,
		config:      appConfig,
		keys:        keyBindings,
	}

	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		screen.SetCursorStyle(app.cursorStyle)
		return false
	})

	keyBindings.OnPendingChanged = func(s string) {
		app.manager.Broadcast(manager.NewSequencePendingChangedMsg(s))
	}

	app.Pages = NewPages(app.manager, app)
	app.Pages.SetStyle(styles)

	return app
}

func (a *App) ReloadKeybindings() error {
	newKeys, err := config.LoadKeybindings(a.config.UI.VimMode)
	if err != nil {
		return err
	}
	a.keys.ReloadKeybidings(newKeys)
	a.keys.Reset()
	return nil
}

func (a *App) SetStyle(styleName string) error {
	a.config.Styles.CurrentStyle = styleName
	err := a.config.UpdateConfig()
	if err != nil {
		return err
	}

	a.styles, err = config.LoadStyles(a.config.Styles.CurrentStyle, a.config.Styles.NerdFont)
	if err != nil {
		return err
	}
	a.styles.LoadMainStyles()
	a.Pages.SetStyle(a.styles)
	a.manager.Broadcast(manager.NewStyleChangedMsg())

	return nil
}

func (a *App) SetCursorStyle(style tcell.CursorStyle) {
	a.cursorStyle = style
}

func (a *App) SnapshotFocus() {
	a.focusStack = append(a.focusStack, a.lastFocusedPrimitive)
}

func (a *App) SetFocus(p tview.Primitive) {
	a.Application.SetFocus(p)
	a.FocusChanged(p)
}

func (a *App) SetFocusOnly(p tview.Primitive) {
	a.Application.SetFocus(p)
	a.FocusChanged(p)
}

func (a *App) RestoreFocus() {
	if len(a.focusStack) == 0 {
		return
	}
	prev := a.focusStack[len(a.focusStack)-1]
	a.focusStack = a.focusStack[:len(a.focusStack)-1]
	a.Application.SetFocus(prev)
	a.FocusChanged(prev)
}

func (a *App) FocusChanged(p tview.Primitive) {
	a.keys.Reset()
	a.manager.Broadcast(manager.NewFocusChangedMsg(p.GetIdentifier()))
	a.lastFocusedPrimitive = p
}

func (a *App) GetDriver() database.Driver {
	return a.driver
}

func (a *App) SetDriver(driver database.Driver) {
	a.driver = driver
}

func (a *App) GetFormatter() database.ValueFormatter {
	return a.formatter
}

func (a *App) SetFormatter(formatter database.ValueFormatter) {
	a.formatter = formatter
}

// GetQuoter returns the identifier quoter for the currently connected driver.
// Falls back to ANSI quoting when no connection is active.
func (a *App) GetQuoter() util.Quoter {
	if a.config != nil {
		if conn := a.config.GetCurrentConnection(); conn != nil {
			if def, ok := database.GetConnector(conn.GetDriver()); ok {
				return def.Quoter
			}
		}
	}
	return util.ANSIQuoter
}

func (a *App) GetManager() *manager.ElementManager {
	return a.manager
}

func (a *App) GetKeys() *config.KeyBindings {
	return a.keys
}

func (a *App) GetStyles() *config.Styles {
	return a.styles
}

func (a *App) GetConfig() *config.Config {
	return a.config
}
