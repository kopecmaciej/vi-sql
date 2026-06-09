package tui

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/build"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	visqlmcp "github.com/kopecmaciej/vi-sql/internal/mcp"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/page"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type App struct {
	*core.App

	connection     *page.Connection
	main           *page.Main
	help           *page.Help
	mcpCancelFunc  context.CancelFunc
	tabRegistry    *manager.TabRegistry
	currentFocusID string
	latestVersion  string
}

func NewApp(appConfig *config.Config) *App {
	coreApp := core.NewApp(appConfig)
	reg := manager.NewTabRegistry()
	mainPage := page.NewMain()
	mainPage.SetRegistry(reg)

	app := &App{
		App:         coreApp,
		connection:  page.NewConnection(),
		main:        mainPage,
		help:        page.NewHelp(),
		tabRegistry: reg,
	}

	return app
}

func (a *App) Init() error {
	a.SetRoot(a.Pages, true).EnableMouse(a.App.GetConfig().UI.Mouse)

	a.App.SetOpenStyleModalFunc(a.ShowStyleChangeModal)
	a.App.SetOpenConnectionPageFunc(a.renderConnection)
	a.App.SetOpenOptionsPageFunc(a.renderOptions)
	a.App.SetToggleMCPFunc(a.toggleMCPServer)

	err := a.help.Init(a.App)
	if err != nil {
		return err
	}
	go a.handleEvents()
	a.setKeybindings()

	if err := a.connection.Init(a.App); err != nil {
		return err
	}
	return nil
}

func (a *App) Run() error {
	a.startWatchdog()
	return a.Application.Run()
}

func (a *App) shutdown() {
	if a.mcpCancelFunc != nil {
		a.mcpCancelFunc()
		log.Debug().Msg("MCP server stopped")
	}

	if driver := a.GetDriver(); driver != nil {
		if err := driver.Close(context.Background()); err != nil {
			log.Error().Err(err).Msg("Failed to close database connection")
		}
	}

	a.GetManager().Stop()
	log.Debug().Msg("Event manager stopped")

	a.Stop()
	log.Debug().Msg("App stopped")
}

// startWatchdog periodically probes the tview event loop, if the loop does not
// respond, all goroutine stacks are dumped to the log. Only active in debug mode.
func (a *App) startWatchdog() {
	if zerolog.GlobalLevel() > zerolog.DebugLevel {
		return
	}
	log.Debug().Msg("Watchdog is running")
	const (
		pingInterval  = 2 * time.Second
		freezeTimeout = 5 * time.Second
	)
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for range ticker.C {
			done := make(chan struct{}, 1)
			go func() {
				a.Application.QueueUpdate(func() { done <- struct{}{} })
			}()
			select {
			case <-done: // event loop is responsive
			case <-time.After(freezeTimeout):
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				log.Error().Msgf("UI freeze detected — event loop unresponsive for >%s:\n%s", freezeTimeout, buf[:n])
			}
		}
	}()
}

func (a *App) setKeybindings() {
	k := a.GetKeys()
	k.SequencesDisabled = a.isTextInputFocused
	a.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && a.isTextInputFocused() {
			return event
		}

		switch {
		case k.Match(k.Global.CloseApp, event):
			a.shutdown()
			return nil
		case k.Match(k.Global.OpenConnection, event):
			a.renderConnection()
			return nil
		case k.Match(k.Global.ChangeStyle, event):
			a.ShowStyleChangeModal()
			return nil
		case k.Match(k.Global.ToggleFooter, event):
			if a.main.App != nil {
				a.main.ToggleFooter()
			}
			return nil
		case k.Match(k.Global.FullScreenHelp, event):
			if a.Pages.HasPage(page.HelpPageId) {
				a.Pages.RemovePage(page.HelpPageId)
				return nil
			}
			a.openHelp()
			return nil
		}
		return event
	}))
}

func (a *App) handleEvents() {
	ch := a.GetManager().Subscribe("App")
	done := a.GetManager().Done()
	for {
		select {
		case event := <-ch:
			if event.Message.Type != manager.FocusChanged {
				continue
			}
			id, ok := event.Message.Data.(tview.Identifier)
			if !ok {
				continue
			}
			a.currentFocusID = string(id)
		case <-done:
			return
		}
	}
}

func (a *App) openHelp() {
	a.help.OpenAt(a.currentFocusID)
	a.Pages.AddPage(page.HelpPageId, a.help, true, true)
}

// isTextInputFocused check whether currently focused primitives is
// raw text input or input/visual vim mode, if yes, single-rune
// events must be pass through untouched.
func (a *App) isTextInputFocused() bool {
	focus := a.GetFocus()
	if focus == nil {
		return false
	}

	switch f := focus.(type) {
	case *tview.InputField, *core.InputField:
		return true
	case *tview.TextView:
		return false
	case *core.TextView:
		return f.IsRawInput()
	case *component.SQLQueryEditor:
		return f.IsInsertMode() || f.IsVisualMode()
	}
	if _, ok := focus.(tview.FormItem); ok {
		return true
	}

	return false
}

func (a *App) connectToDatabase() error {
	currConn := a.App.GetConfig().GetCurrentConnection()
	if currConn == nil {
		return nil
	}

	if old := a.App.GetDriver(); old != nil {
		_ = old.Close(context.Background())
	}

	driver, formatter, err := database.NewDriver(currConn)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		return err
	}
	log.Info().Str("connection", currConn.Name).Msg("Connected to database")
	a.SetDriver(driver)
	a.SetFormatter(formatter)

	a.startMCPServer(driver)
	return nil
}

func (a *App) startMCPServer(driver database.Driver) {
	cfg := a.App.GetConfig().MCP
	if !cfg.Enabled {
		return
	}

	if a.mcpCancelFunc != nil {
		a.mcpCancelFunc()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.mcpCancelFunc = cancel

	srv := visqlmcp.New(driver, cfg, a.App.GetManager(), a.tabRegistry)
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Error().Err(err).Msg("MCP server error")
		}
	}()

	a.App.SetMCPEnabled(true)
	a.App.GetManager().Broadcast(manager.NewMCPStateChangedMsg(true))
}

func (a *App) toggleMCPServer() {
	cfg := a.App.GetConfig()
	cfg.MCP.Enabled = !cfg.MCP.Enabled
	_ = cfg.UpdateConfig()

	if cfg.MCP.Enabled {
		if a.App.GetDriver() != nil {
			a.startMCPServer(a.App.GetDriver())
		}
	} else {
		if a.mcpCancelFunc != nil {
			a.mcpCancelFunc()
			a.mcpCancelFunc = nil
		}
		a.App.SetMCPEnabled(false)
		a.App.GetManager().Broadcast(manager.NewMCPStateChangedMsg(false))
	}
}

func (a *App) Render() {
	if pending := getPendingChangelog(a.App.GetConfig().Version); len(pending) > 0 {
		a.showChangelogModal(pending)
		return
	}
	a.updateConfigVersion()
	a.gateOnMasterPassword(a.continueStartup)
}

// gateOnMasterPassword shows the master-password modal (setup or unlock,
// depending on whether a wrapped key already exists) if in master method.
func (a *App) gateOnMasterPassword(next func()) {
	cfg := a.App.GetConfig()
	if cfg.Security.Method != config.SecurityMethodMaster || config.EncryptionKey != "" {
		next()
		return
	}

	mode := modal.MasterModeUnlock
	if !cfg.IsMasterConfigured() {
		mode = modal.MasterModeSetup
	}
	m := modal.NewMasterPasswordModal(mode)
	if err := m.Init(a.App); err != nil {
		modal.ShowError(a.Pages, "Failed to init master password modal", err)
		return
	}
	m.SetOnDone(func(err error) {
		if err != nil {
			a.Stop()
			return
		}
		next()
	})
	m.Render()
}

func (a *App) continueStartup() {
	if err := applyPendingConnect(a.App.GetConfig()); err != nil {
		modal.ShowError(a.Pages, "Failed to add --connect connection", err)
		a.App.GetConfig().ShowConnectionPage = true
	}
	switch {
	case a.App.GetConfig().ShowOptionsPage:
		a.renderOptionsOnStartup()
	case a.App.GetConfig().GetCurrentConnection() == nil, a.App.GetConfig().ShowConnectionPage:
		a.renderConnection()
	default:
		a.initAndRenderMain()
	}
	go a.checkForUpdate()
}

// applyPendingConnect adds a connection passed via --connect.
func applyPendingConnect(cfg *config.Config) error {
	if cfg.PendingConnect == "" {
		return nil
	}
	raw := cfg.PendingConnect
	cfg.PendingConnect = ""

	// PendingConnect is stored as "name=dsn" by cmd.go after name resolution.
	name, dsn, _ := strings.Cut(raw, "=")
	conn, err := database.BuildConfigFromDSN(name, dsn)
	if err != nil {
		return err
	}
	if existing, _ := cfg.GetConnectionByName(conn.Name); existing == nil {
		if err := cfg.AddConnection(conn); err != nil {
			return err
		}
	}
	return cfg.SetCurrentConnection(conn.Name)
}

func getPendingChangelog(lastVersion string) []util.ChangelogEntry {
	return util.FilterPendingEntries(lastVersion, modal.LoadChangelog())
}

func (a *App) showChangelogModal(entries []util.ChangelogEntry) {
	m := modal.NewChangelog(entries)
	if err := m.Init(a.App); err != nil {
		log.Error().Err(err).Msg("Failed to init changelog modal")
		a.updateConfigVersion()
		a.continueStartup()
		return
	}

	m.SetOnProceed(func() {
		if err := util.RunMigrations(entries); err != nil {
			log.Error().Err(err).Msg("Migration failed")
			modal.ShowError(a.Pages, "Migration failed", err)
			return
		}
		a.updateConfigVersion()
		a.Pages.RemovePage(modal.ChangelogModalId)
		a.continueStartup()
	})

	m.SetOnQuit(func() {
		os.Exit(0)
	})

	m.Render()
}

func (a *App) updateConfigVersion() {
	normalized := util.NormalizeVersion(build.Version)
	cfg := a.App.GetConfig()
	if cfg.Version != normalized {
		if err := cfg.UpdateVersion(normalized); err != nil {
			log.Error().Err(err).Msg("Failed to update config version")
		}
	}
}

func (a *App) checkForUpdate() {
	latest := util.FetchLatestVersion(context.Background())
	if latest == "" || !util.IsNewerVersion(build.Version, latest) {
		return
	}
	a.Application.QueueUpdateDraw(func() {
		a.latestVersion = latest
		if a.main.App != nil {
			a.main.SetUpdateHandler(func() { a.startSelfUpdate(latest) })
		}
	})
}

func (a *App) startSelfUpdate(tag string) {
	confirmModal := modal.NewConfirm("SelfUpdateConfirm")
	if err := confirmModal.Init(a.App); err != nil {
		modal.ShowError(a.Pages, "Failed to init update modal", err)
		return
	}
	confirmModal.SetText(fmt.Sprintf("Update vi-sql from %s to %s?", build.Version, tag))
	confirmModal.SetConfirmButtonLabel("Update")
	confirmModal.SetOnConfirm(func() {
		a.Pages.RemovePage("SelfUpdateConfirm")

		progress := tview.NewModal()
		progress.SetTitle(" Updating vi-sql ")
		progress.SetText("Starting update…")
		progress.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
		a.Pages.AddPage("SelfUpdateProgress", progress, true, true)

		go func() {
			err := util.ApplyUpdate(context.Background(), tag, func(step string) {
				a.Application.QueueUpdateDraw(func() { progress.SetText(step) })
			})
			a.Application.QueueUpdateDraw(func() {
				a.Pages.RemovePage("SelfUpdateProgress")
				if err != nil {
					modal.ShowError(a.Pages, "Update failed", err)
					return
				}
				done := tview.NewModal()
				done.SetTitle(" Update complete ")
				done.SetText(fmt.Sprintf("Updated to %s! Please restart vi-sql.", tag))
				done.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
				done.AddButtons([]string{"Exit"})
				done.SetDoneFunc(func(_ int, _ string) { os.Exit(0) })
				a.Pages.AddPage("SelfUpdateDone", done, true, true)
			})
		}()
	})
	a.Pages.AddPage("SelfUpdateConfirm", confirmModal, true, true)
	a.App.SetFocusOnly(confirmModal)
}

func (a *App) initAndRenderMain() {
	if err := a.connectToDatabase(); err != nil {
		a.renderConnection()
		if _, ok := err.(*util.EncryptionError); ok {
			modal.ShowError(a.Pages, "Encryption error occurred", err)
		} else {
			modal.ShowError(a.Pages, "Error while connecting to database", err)
		}
		return
	}

	if a.main.App != nil || a.main.Driver != nil {
		a.main.UpdateDriver(a.GetDriver())
	} else {
		if err := a.main.Init(a.App); err != nil {
			log.Fatal().Err(err).Msg("Error while initializing main view")
			os.Exit(1)
		}
	}

	a.main.Render()

	if a.latestVersion != "" {
		a.main.SetUpdateHandler(func() { a.startSelfUpdate(a.latestVersion) })
	}

	if jumpInto := a.GetConfig().JumpInto; jumpInto != "" {
		// Defer past the first draw so TabBar sees the real terminal width
		go a.Application.QueueUpdateDraw(func() {
			if err := a.jumpToTable(jumpInto); err != nil {
				modal.ShowError(a.Pages, "Unable to jump into the schema/table", err)
			}
		})
	}
}

func (a *App) renderConnection() {
	a.connection.SetOnSubmitFunc(func() {
		a.Pages.RemovePage(a.connection.GetIdentifier())
		a.initAndRenderMain()
	})

	a.Pages.AddPage(a.connection.GetIdentifier(), a.connection, true, true)
	a.connection.Render()
}

func (a *App) renderOptionsOnStartup() {
	opts := page.NewOptions()
	if err := opts.Init(a.App); err != nil {
		a.Pages.AddPage(opts.GetIdentifier(), opts, true, true)
		modal.ShowError(a.Pages, "Error while rendering options page", err)
		return
	}
	opts.SetOnSubmitFunc(func() {
		a.Pages.RemovePage(opts.GetIdentifier())
		if err := a.App.ReloadKeybindings(); err != nil {
			log.Error().Err(err).Msg("Failed to reload keybindings")
		}
		a.renderConnection()
	})
	a.Pages.AddPage(opts.GetIdentifier(), opts, true, true)
	opts.Render()
}

func (a *App) renderOptions() {
	opts := page.NewOptions()
	if err := opts.Init(a.App); err != nil {
		a.Pages.AddPage(opts.GetIdentifier(), opts, true, true)
		modal.ShowError(a.Pages, "Error while rendering options page", err)
		return
	}
	opts.SetOnSubmitFunc(func() {
		a.Pages.RemovePage(opts.GetIdentifier())
		if a.App.GetConfig().MCP.Enabled && a.App.GetDriver() != nil {
			a.startMCPServer(a.App.GetDriver())
		}
		if err := a.App.ReloadKeybindings(); err != nil {
			log.Error().Err(err).Msg("Failed to reload keybindings")
		}
		a.App.GetManager().Broadcast(manager.NewConfigChangedMsg())
	})
	a.Pages.AddPage(opts.GetIdentifier(), opts, true, true)
	opts.Render()
}

func (a *App) ShowStyleChangeModal() {
	styleChangeModal := modal.NewStyleChangeModal()
	if err := styleChangeModal.Init(a.App); err != nil {
		modal.ShowError(a.Pages, "Error while initializing style change modal", err)
	}
	styleChangeModal.Render()
	styleChangeModal.SetApplyStyle(func(styleName string) error {
		return a.SetStyle(styleName)
	})
}

func (a *App) jumpToTable(jumpTo string) error {
	schema, table, err := parseJumpTarget(jumpTo)
	if err != nil {
		return err
	}
	return a.main.JumpToTable(schema, table)
}

func parseJumpTarget(jumpTo string) (schema, table string, err error) {
	parts := strings.SplitN(jumpTo, ".", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid jump target %q: expected schema.table", jumpTo)
	}
	schema = strings.TrimSpace(parts[0])
	table = strings.TrimSpace(parts[1])
	if schema == "" || table == "" {
		return "", "", fmt.Errorf("invalid jump target %q: schema and table must not be empty", jumpTo)
	}
	return schema, table, nil
}
