package tui

import (
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/postgres"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/page"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type App struct {
	*core.App

	connection *page.Connection
	main       *page.Main
	help       *page.Help
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

	err := a.help.Init(a.App)
	if err != nil {
		return err
	}
	a.setKeybindings()

	a.connection.Init(a.App)
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
		case a.GetKeys().Contains(a.GetKeys().Global.ShowStyleModal, event.Name()):
			a.ShowStyleChangeModal()
			return nil
		case a.GetKeys().Contains(a.GetKeys().Global.ToggleHeader, event.Name()):
			if a.main.App != nil {
				a.main.ToggleHeader()
			}
			return nil
		case a.GetKeys().Contains(a.GetKeys().Global.ToggleFullScreenHelp, event.Name()):
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

	if strings.Contains(identifier, "Bar") || strings.Contains(identifier, "Input") {
		return true
	}

	_, isInputField := focus.(*tview.InputField)
	_, isCustomInputField := focus.(*core.InputField)
	_, isFormItem := focus.(tview.FormItem)

	return isInputField || isCustomInputField || isFormItem
}

func (a *App) connectToDatabase() error {
	currConn := a.App.GetConfig().GetCurrentConnection()
	if currConn == nil {
		return nil
	}

	client := postgres.NewClient(currConn)
	if err := client.Connect(); err != nil {
		log.Error().Err(err).Msg("Failed to connect to PostgreSQL")
		return err
	}
	if err := client.Ping(); err != nil {
		log.Error().Err(err).Msg("Failed to ping PostgreSQL")
		return err
	}
	a.SetDriver(postgres.NewDao(client))
	return nil
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
	a.Pages.AddPage(a.main.GetIdentifier(), a.main, true, true)

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
