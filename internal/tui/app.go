package tui

import (
	"context"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	visqlmcp "github.com/kopecmaciej/vi-sql/internal/mcp"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/page"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type App struct {
	*core.App

	connection    *page.Connection
	main          *page.Main
	help          *page.Help
	mcpCancelFunc context.CancelFunc
}

func NewApp(appConfig *config.Config) *App {
	coreApp := core.NewApp(appConfig)

	app := &App{
		App:        coreApp,
		connection: page.NewConnection(),
		main:       page.NewMain(),
		help:       page.NewHelp(),
	}

	return app
}

func (a *App) Init() error {
	a.SetRoot(a.Pages, true).EnableMouse(true)

	a.App.SetOpenStyleModalFunc(a.ShowStyleChangeModal)
	a.App.SetOpenConnectionPageFunc(a.renderConnection)
	a.App.SetToggleMCPFunc(a.toggleMCPServer)

	err := a.help.Init(a.App)
	if err != nil {
		return err
	}
	a.setKeybindings()

	if err := a.connection.Init(a.App); err != nil {
		return err
	}
	return nil
}

func (a *App) Run() error {
	return a.Application.Run()
}

func (a *App) setKeybindings() {
	a.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if a.shouldHandleRune(event) {
			return event
		}

		switch {
		case a.GetKeys().Contains(a.GetKeys().Global.CloseApp, event.Name()):
			a.Stop()
			return nil
		case a.GetKeys().Contains(a.GetKeys().Global.OpenConnection, event.Name()):
			a.renderConnection()
			return nil
		case a.GetKeys().Contains(a.GetKeys().Global.ChangeStyle, event.Name()):
			a.ShowStyleChangeModal()
			return nil
		case a.GetKeys().Contains(a.GetKeys().Global.ToggleFooter, event.Name()):
			if a.main.App != nil {
				a.main.ToggleFooter()
			}
			return nil
		case a.GetKeys().Contains(a.GetKeys().Global.FullScreenHelp, event.Name()):
			if a.Pages.HasPage(page.HelpPageId) {
				a.Pages.RemovePage(page.HelpPageId)
				return nil
			}
			a.help.Render()
			a.Pages.AddPage(page.HelpPageId, a.help, true, true)
			return nil
		}
		return event
	})
}

func (a *App) shouldHandleRune(event *tcell.EventKey) bool {
	if !strings.HasPrefix(event.Name(), "Rune") {
		return false
	}

	focus := a.GetFocus()
	identifier := string(focus.GetIdentifier())

	// TODO: find better way of handling this focus problem in input fields
	if strings.Contains(identifier, "Bar") || strings.Contains(identifier, "Input") || strings.Contains(identifier, "CreateTable") {
		return true
	}

	_, isInputField := focus.(*tview.InputField)
	_, isCustomInputField := focus.(*core.InputField)
	_, isFormItem := focus.(tview.FormItem)
	_, isInputModal := focus.(*primitives.InputModal)

	return isInputField || isCustomInputField || isFormItem || isInputModal
}

func (a *App) connectToDatabase() error {
	currConn := a.App.GetConfig().GetCurrentConnection()
	if currConn == nil {
		return nil
	}

	driver, formatter, err := database.NewDriver(currConn)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		return err
	}
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

	srv := visqlmcp.New(driver, cfg)
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Error().Err(err).Msg("MCP server error")
		}
	}()

	a.App.SetMCPEnabled(true)
	a.App.GetManager().Broadcast(manager.EventMsg{
		Message: manager.Message{Type: manager.MCPStateChanged, Data: true},
	})
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
		a.App.GetManager().Broadcast(manager.EventMsg{
			Message: manager.Message{Type: manager.MCPStateChanged, Data: false},
		})
	}
}

func (a *App) Render() {
	switch {
	case a.App.GetConfig().ShowWelcomePage:
		a.renderWelcome()
	case a.App.GetConfig().GetCurrentConnection() == nil, a.App.GetConfig().ShowConnectionPage:
		a.renderConnection()
	default:
		a.initAndRenderMain()
	}
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

	if jumpInto := a.GetConfig().JumpInto; jumpInto != "" {
		if err := a.jumpToTable(jumpInto); err != nil {
			modal.ShowError(a.Pages, "Unable to jump into the schema/table", err)
		}
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

func (a *App) renderWelcome() {
	welcome := page.NewWelcome()
	if err := welcome.Init(a.App); err != nil {
		a.Pages.AddPage(welcome.GetIdentifier(), welcome, true, true)
		modal.ShowError(a.Pages, "Error while rendering welcome page", err)
		return
	}
	welcome.SetOnSubmitFunc(func() {
		a.Pages.RemovePage(welcome.GetIdentifier())
		a.renderConnection()
	})
	a.Pages.AddPage(welcome.GetIdentifier(), welcome, true, true)
	welcome.Render()
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
	parts := strings.Split(jumpTo, "/")
	schemaName := strings.TrimSpace(parts[0])
	tableName := strings.TrimSpace(parts[1])

	return a.main.JumpToTable(schemaName, tableName)
}
