package page

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	ConnectionFormPageId = "ConnectionForm"
)

// ConnectionForm is a full-page add/edit form for a single SQL connection.
// Pass a non-nil conn to pre-populate the form in edit mode.
type ConnectionForm struct {
	*core.BaseElement
	*core.Flex

	form *core.Form

	editConn      *config.SQLConfig // nil == add mode
	editOrigName  string
	currentDriver string

	onSave   func()
	onCancel func()
}

func NewConnectionForm(conn *config.SQLConfig) *ConnectionForm {
	cf := &ConnectionForm{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		form:        core.NewForm(),
	}

	cf.SetIdentifier(ConnectionFormPageId)

	if conn != nil {
		cf.editConn = conn
		cf.editOrigName = conn.Name
		cf.currentDriver = conn.GetDriver()
	} else {
		cf.currentDriver = "postgres"
	}

	return cf
}

func (cf *ConnectionForm) SetOnSaveFunc(fn func()) {
	cf.onSave = fn
}

func (cf *ConnectionForm) SetOnCancelFunc(fn func()) {
	cf.onCancel = fn
}

func (cf *ConnectionForm) Init(app *core.App) {
	cf.App = app
	cf.setStyle()
}

func (cf *ConnectionForm) setStyle() {
	cf.SetStyle(cf.App.GetStyles())
	cf.form.SetStyle(cf.App.GetStyles())

	style := &cf.App.GetStyles().Connection

	cf.form.SetFieldTextColor(style.FormInputColor.Color())
	cf.form.SetFieldBackgroundColor(style.FormInputBackgroundColor.Color())
	cf.form.SetLabelColor(style.FormLabelColor.Color())
}

func (cf *ConnectionForm) Render() {
	cf.Clear()

	title := " Add new connection "
	if cf.editConn != nil {
		title = " Edit connection "
	}
	cf.form.SetTitle(title)
	cf.form.SetBorder(true)

	cf.buildForm(cf.currentDriver)

	cf.AddItem(tview.NewBox(), 0, 1, false)
	cf.AddItem(cf.form, 0, 4, true)
	cf.AddItem(tview.NewBox(), 0, 1, false)

	cf.App.SetFocus(cf.form)
}

// buildForm clears and rebuilds the form for the given driver.
func (cf *ConnectionForm) buildForm(driver string) {
	cf.currentDriver = driver
	cf.form.Clear(false)

	// --- Driver selector (add mode only; locked in edit mode) ---
	drivers := []string{"postgres", "sqlite"}
	driverIdx := 0
	for i, d := range drivers {
		if d == driver {
			driverIdx = i
			break
		}
	}
	if cf.editConn != nil {
		// Show driver as read-only text in edit mode — changing driver on an
		// existing connection would invalidate all saved fields.
		cf.form.AddTextView("Driver", driver, 0, 1, true, false)
	} else {
		cf.form.AddDropDown("Driver", drivers, driverIdx, func(option string, _ int) {
			if option != cf.currentDriver {
				cf.buildForm(option)
				cf.App.SetFocus(cf.form)
			}
		})
	}

	// --- Name ---
	nameVal := ""
	if cf.editConn != nil {
		nameVal = cf.editConn.Name
	}
	cf.form.AddInputField("Name", nameVal, 0, nil, nil)

	// --- Driver-specific connection fields ---
	switch driver {
	case "sqlite":
		fileVal := ""
		if cf.editConn != nil {
			fileVal = cf.editConn.DSN
		}
		cf.form.AddInputField("File path", fileVal, 0, nil, nil)
		cf.form.GetFormItemByLabel("File path").(*tview.InputField).SetClipboard(util.GetClipboard())

	default: // postgres
		dsnVal := "postgresql://"
		if cf.editConn != nil && cf.editConn.DSN != "" {
			dsnVal = cf.editConn.DSN
		}
		cf.form.AddTextArea("DSN", dsnVal, 0, 3, 0, nil)
		cf.form.GetFormItemByLabel("DSN").(*tview.TextArea).SetClipboard(util.GetClipboard())
		cf.form.AddTextView("Example", "postgresql://user:pass@host:5432/db?...", 0, 1, true, false)
		pasteKey := cf.App.GetKeys().InputBar.Paste.String()
		cf.form.AddTextView("Info", fmt.Sprintf("Type/paste(%s) DSN, $ENV or use form", pasteKey), 0, 1, true, false)
		cf.form.AddTextView(" ", "----------------------------------------", 0, 1, true, false)

		hostVal, portVal, userVal, passVal, dbVal := "", "5432", "", "", ""
		sslIdx := 0
		if cf.editConn != nil {
			hostVal = cf.editConn.Host
			if cf.editConn.Port > 0 {
				portVal = fmt.Sprintf("%d", cf.editConn.Port)
			}
			userVal = cf.editConn.Username
			passVal = cf.editConn.Password
			dbVal = cf.editConn.Database
			sslModes := []string{"disable", "require", "verify-ca", "verify-full", "prefer", "allow"}
			for i, m := range sslModes {
				if m == cf.editConn.SSLMode {
					sslIdx = i
					break
				}
			}
		}

		cf.form.AddInputField("Host", hostVal, 0, nil, nil)
		cf.form.AddInputField("Port", portVal, 0, nil, nil)
		cf.form.AddInputField("Username", userVal, 0, nil, nil)
		cf.form.AddPasswordField("Password", passVal, 0, '*', nil)
		cf.form.AddInputField("Database", dbVal, 0, nil, nil)
		cf.form.AddDropDown("SSL Mode", []string{"disable", "require", "verify-ca", "verify-full", "prefer", "allow"}, sslIdx, nil)

		cf.form.GetFormItemByLabel("Host").(*tview.InputField).SetClipboard(util.GetClipboard())
		cf.form.GetFormItemByLabel("Port").(*tview.InputField).SetClipboard(util.GetClipboard())
		cf.form.GetFormItemByLabel("Username").(*tview.InputField).SetClipboard(util.GetClipboard())
		cf.form.GetFormItemByLabel("Password").(*tview.InputField).SetClipboard(util.GetClipboard())
		cf.form.GetFormItemByLabel("Database").(*tview.InputField).SetClipboard(util.GetClipboard())
	}

	// --- Timeout (postgres only) ---
	if driver == "postgres" {
		timeoutVal := "5"
		if cf.editConn != nil && cf.editConn.Timeout > 0 {
			timeoutVal = fmt.Sprintf("%d", cf.editConn.Timeout)
		}
		cf.form.AddInputField("Timeout", timeoutVal, 0, nil, nil)
	}

	// --- Options (always shown) ---
	defaultSchema := ""
	rowLimit := ""
	confirmActionsIdx := 0
	if cf.editConn != nil {
		defaultSchema = cf.editConn.Options.DefaultSchema
		if cf.editConn.Options.Limit != nil {
			rowLimit = fmt.Sprintf("%d", *cf.editConn.Options.Limit)
		}
		if cf.editConn.Options.AlwaysConfirmActions != nil && !*cf.editConn.Options.AlwaysConfirmActions {
			confirmActionsIdx = 1
		}
	}
	cf.form.AddTextView("─── Options", "", 0, 1, true, false)
	cf.form.AddInputField("Default schema", defaultSchema, 0, nil, nil)
	cf.form.AddInputField("Row limit", rowLimit, 0, nil, nil)
	cf.form.AddDropDown("Confirm actions", []string{"yes", "no"}, confirmActionsIdx, nil)

	// --- Buttons ---
	saveLabel := "Save"
	if cf.editConn != nil {
		saveLabel = "Update"
	}
	saveKey := cf.App.GetKeys().Connection.ConnectionForm.SaveConnection.String()
	cf.form.AddTextView("Save with:", fmt.Sprintf("%s or click", saveKey), 0, 1, true, false)
	cf.form.AddButton(saveLabel, cf.save)
	cf.form.AddButton("Cancel", cf.cancel)

	// --- Keybindings ---
	cf.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		k := cf.App.GetKeys()
		switch {
		case event.Key() == tcell.KeyEscape:
			cf.cancel()
			return nil
		case k.Contains(k.Connection.ConnectionForm.SaveConnection, event.Name()):
			_, buttonIdx := cf.form.GetFocusedItemIndex()
			if buttonIdx >= 0 && buttonIdx < cf.form.GetButtonCount() {
				if cf.form.GetButton(buttonIdx).GetLabel() == "Cancel" {
					return event
				}
			}
			cf.save()
			return nil
		}
		return event
	})
	cf.form.ApplyFormNavKeys(cf.App.GetKeys())
	cf.form.ApplyDropdownNavKeys(cf.App.GetKeys())
}

func (cf *ConnectionForm) save() {
	driver := cf.currentDriver
	name := cf.form.GetFormItemByLabel("Name").(*tview.InputField).GetText()

	opts, err := cf.collectOptions()
	if err != nil {
		showError(cf.App.Pages, "Invalid options", err)
		return
	}

	var sqlCfg *config.SQLConfig
	var saveErr error

	switch driver {
	case "sqlite":
		filePath := cf.form.GetFormItemByLabel("File path").(*tview.InputField).GetText()
		if filePath == "" {
			showError(cf.App.Pages, "File path is required", fmt.Errorf("please enter a SQLite database file path"))
			return
		}
		if name == "" {
			name = filePath
		}
		sqlCfg = &config.SQLConfig{
			Driver:  "sqlite",
			Name:    name,
			DSN:     filePath,
			Options: opts,
		}

	default: // postgres
		timeout := 5
		if t := cf.form.GetFormItemByLabel("Timeout").(*tview.InputField).GetText(); t != "" {
			parsed, err := strconv.Atoi(t)
			if err != nil {
				showError(cf.App.Pages, "Timeout must be a number", err)
				return
			}
			timeout = parsed
		}

		dsn := cf.form.GetFormItemByLabel("DSN").(*tview.TextArea).GetText()
		trimmedDSN := strings.TrimSpace(dsn)

		if trimmedDSN != "postgresql://" && trimmedDSN != "postgres://" && trimmedDSN != "" {
			if name == "" {
				name = trimmedDSN
			}
			sqlCfg = &config.SQLConfig{
				Driver:  "postgres",
				Name:    name,
				DSN:     trimmedDSN,
				Timeout: timeout,
				Options: opts,
			}
			if strings.HasPrefix(trimmedDSN, "$") {
				// env var reference — store as-is
			} else {
				if !strings.HasPrefix(trimmedDSN, "postgres://") && !strings.HasPrefix(trimmedDSN, "postgresql://") {
					showError(cf.App.Pages, "Invalid DSN", fmt.Errorf("DSN must start with postgres:// or postgresql://"))
					return
				}
				parsed, err := util.ParsePostgresDSN(trimmedDSN)
				if err != nil || parsed.Host == "" {
					showError(cf.App.Pages, "Invalid DSN", fmt.Errorf("could not parse host from DSN — check format: postgresql://user:pass@host:5432/db"))
					return
				}
				// Use DSN-based save
				if cf.editConn != nil {
					saveErr = cf.App.GetConfig().UpdateConnectionFromDSN(cf.editOrigName, sqlCfg)
				} else {
					saveErr = cf.App.GetConfig().AddConnectionFromDSN(sqlCfg)
				}
				if saveErr != nil {
					cf.showSaveError(saveErr)
					return
				}
				if cf.onSave != nil {
					cf.onSave()
				}
				return
			}
		} else {
			// Use form fields
			host := cf.form.GetFormItemByLabel("Host").(*tview.InputField).GetText()
			port := cf.form.GetFormItemByLabel("Port").(*tview.InputField).GetText()
			intPort, err := strconv.Atoi(port)
			if err != nil {
				showError(cf.App.Pages, "Port must be a number", err)
				return
			}
			username := cf.form.GetFormItemByLabel("Username").(*tview.InputField).GetText()
			password := cf.form.GetFormItemByLabel("Password").(*tview.InputField).GetText()
			database := cf.form.GetFormItemByLabel("Database").(*tview.InputField).GetText()
			_, sslMode := cf.form.GetFormItemByLabel("SSL Mode").(*tview.DropDown).GetCurrentOption()

			if name == "" {
				name = host + ":" + port
			}
			sqlCfg = &config.SQLConfig{
				Driver:   "postgres",
				Name:     name,
				Host:     host,
				Port:     intPort,
				Username: username,
				Password: password,
				Database: database,
				SSLMode:  sslMode,
				Timeout:  timeout,
				Options:  opts,
			}
		}
	}

	if cf.editConn != nil {
		saveErr = cf.App.GetConfig().UpdateConnection(cf.editOrigName, sqlCfg)
	} else {
		saveErr = cf.App.GetConfig().AddConnection(sqlCfg)
	}

	if saveErr != nil {
		cf.showSaveError(saveErr)
		return
	}

	if cf.onSave != nil {
		cf.onSave()
	}
}

func (cf *ConnectionForm) collectOptions() (config.SQLOptions, error) {
	opts := config.SQLOptions{}

	opts.DefaultSchema = cf.form.GetFormItemByLabel("Default schema").(*tview.InputField).GetText()

	limitStr := cf.form.GetFormItemByLabel("Row limit").(*tview.InputField).GetText()
	if limitStr != "" {
		n, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil {
			return opts, fmt.Errorf("row limit must be a number")
		}
		opts.Limit = &n
	}

	_, confirmStr := cf.form.GetFormItemByLabel("Confirm actions").(*tview.DropDown).GetCurrentOption()
	boolVal := confirmStr == "yes"
	opts.AlwaysConfirmActions = &boolVal

	return opts, nil
}

func (cf *ConnectionForm) showSaveError(err error) {
	action := "save"
	if cf.editConn != nil {
		action = "update"
	}
	showError(cf.App.Pages, fmt.Sprintf("Failed to %s connection", action), err)
}

func (cf *ConnectionForm) cancel() {
	if cf.onCancel != nil {
		cf.onCancel()
	}
}
