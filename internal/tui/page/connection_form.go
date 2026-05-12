package page

import (
	"fmt"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
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

	form   *core.Form
	footer *component.Footer

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
		footer:      component.NewFooter(),
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

func (cf *ConnectionForm) Init(app *core.App) error {
	cf.App = app
	if err := cf.footer.Init(app); err != nil {
		return err
	}
	cf.setStyle()
	return nil
}

func (cf *ConnectionForm) setStyle() {
	styles := cf.App.GetStyles()
	cf.SetStyle(styles)
	cf.form.SetStyle(styles)

	cf.form.SetFieldTextColor(styles.Global.TextColor.Color())
	cf.form.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	cf.form.SetLabelColor(styles.Global.SecondaryTextColor.Color())
	cf.footer.SetCentered(true)
}

func (cf *ConnectionForm) Render() {
	cf.Clear()
	cf.SetDirection(tview.FlexRow)

	title := " Add new connection "
	if cf.editConn != nil {
		title = " Edit connection "
	}
	cf.form.SetTitle(title)
	cf.form.SetBorder(true)

	cf.buildForm(cf.currentDriver)

	centerFlex := tview.NewFlex()
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)
	centerFlex.AddItem(cf.form, 0, 4, true)
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)

	cf.AddItem(centerFlex, 0, 1, true)
	cf.renderFooter()
	cf.AddItem(cf.footer, 2, 0, false)

	cf.App.SetFocus(cf.form)
}

func (cf *ConnectionForm) renderFooter() {
	k := cf.App.GetKeys()
	cf.footer.SetKeys([]config.Key{
		k.Navigation.FocusUp,
		k.Navigation.FocusDown,
		k.Common.Confirm,
		{Keys: []string{"Esc"}, Description: "cancel"},
	})
}

// buildForm clears and rebuilds the form for the given driver.
func (cf *ConnectionForm) buildForm(driver string) {
	cf.currentDriver = driver
	cf.form.Clear(true)

	// Driver selector (locked to read-only text in edit mode).
	drivers := database.ListConnectors()
	driverIdx := 0
	for i, d := range drivers {
		if d == driver {
			driverIdx = i
			break
		}
	}
	if cf.editConn != nil {
		cf.form.AddTextView("Driver", driver, 0, 1, true, false)
	} else {
		cf.form.AddDropDown("Driver", drivers, driverIdx, func(option string, _ int) {
			if option != cf.currentDriver {
				cf.buildForm(option)
				cf.App.SetFocus(cf.form)
			}
		})
	}

	// Name is common to all drivers.
	nameVal := ""
	if cf.editConn != nil {
		nameVal = cf.editConn.Name
	}
	cf.form.AddInputField("Name", nameVal, 0, nil, nil)

	// Driver-specific fields from the connector spec.
	def, ok := database.GetConnector(driver)
	if !ok {
		return
	}

	var fillValues map[string]string
	if cf.editConn != nil {
		fillValues = def.PreFill(cf.editConn)
	}

	for _, spec := range def.FormSpec {
		value := spec.Default
		if fv, exists := fillValues[spec.Label]; exists && fv != "" {
			value = fv
		}
		switch spec.Kind {
		case database.FieldInput:
			cf.form.AddInputField(spec.Label, value, 0, nil, nil)
			if spec.Clipboard {
				cf.form.GetFormItemByLabel(spec.Label).(*tview.InputField).SetClipboard(util.GetClipboard())
			}
		case database.FieldPassword:
			cf.form.AddPasswordField(spec.Label, value, 0, '*', nil)
			if spec.Clipboard {
				cf.form.GetFormItemByLabel(spec.Label).(*tview.InputField).SetClipboard(util.GetClipboard())
			}
			if cf.App.GetConfig().Security.Method == config.SecurityMethodOff {
				cf.form.AddTextView("", "[red]⚠ password will be stored unencrypted[-]", 0, 1, true, false)
			}
		case database.FieldTextArea:
			rows := spec.Rows
			if rows == 0 {
				rows = 3
			}
			cf.form.AddTextArea(spec.Label, value, 0, rows, 0, nil)
			if spec.Clipboard {
				cf.form.GetFormItemByLabel(spec.Label).(*tview.TextArea).SetClipboard(util.GetClipboard())
			}
		case database.FieldDropDown:
			idx := 0
			for i, opt := range spec.Options {
				if opt == value {
					idx = i
					break
				}
			}
			cf.form.AddDropDown(spec.Label, spec.Options, idx, nil)
		case database.FieldLabel:
			cf.form.AddTextView(spec.Label, value, 0, 1, true, false)
		}
	}

	// Options section is common to all drivers.
	rowLimit := ""
	confirmActionsIdx := 0
	if cf.editConn != nil {
		if cf.editConn.Options.Limit != nil {
			rowLimit = fmt.Sprintf("%d", *cf.editConn.Options.Limit)
		}
		if cf.editConn.Options.AlwaysConfirmActions != nil && !*cf.editConn.Options.AlwaysConfirmActions {
			confirmActionsIdx = 1
		}
	}
	cf.form.AddTextView("─── Options", "", 0, 1, true, false)
	cf.form.AddInputField("Row limit", rowLimit, 0, nil, nil)
	cf.form.AddDropDown("Confirm actions", []string{"yes", "no"}, confirmActionsIdx, nil)

	saveLabel := "Save"
	if cf.editConn != nil {
		saveLabel = "Update"
	}
	cf.form.AddButton(saveLabel, cf.save)
	cf.form.AddButton("Cancel", cf.cancel)

	k := cf.App.GetKeys()
	cf.form.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			cf.cancel()
			return nil
		case k.Match(k.Common.Confirm, event):
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
	}))
	cf.form.ApplyFormNavKeys(cf.App.GetKeys())
	cf.form.ApplyDropdownNavKeys(cf.App.GetKeys())
}

func (cf *ConnectionForm) save() {
	def, ok := database.GetConnector(cf.currentDriver)
	if !ok {
		modal.ShowError(cf.App.Pages, "Unknown driver", fmt.Errorf("driver %q not registered", cf.currentDriver))
		return
	}

	opts, err := cf.collectOptions()
	if err != nil {
		modal.ShowError(cf.App.Pages, "Invalid options", err)
		return
	}

	fields := cf.collectFields(def.FormSpec)
	fields["Name"] = cf.form.GetFormItemByLabel("Name").(*tview.InputField).GetText()

	sqlCfg, err := def.BuildConfig(fields, cf.editConn)
	if err != nil {
		modal.ShowError(cf.App.Pages, "Invalid connection", err)
		return
	}
	sqlCfg.Options = opts

	var saveErr error
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

// collectFields reads the current value of every interactive field in the spec.
func (cf *ConnectionForm) collectFields(spec []database.FieldSpec) map[string]string {
	fields := make(map[string]string, len(spec))
	for _, s := range spec {
		item := cf.form.GetFormItemByLabel(s.Label)
		if item == nil {
			continue
		}
		switch s.Kind {
		case database.FieldInput, database.FieldPassword:
			fields[s.Label] = item.(*tview.InputField).GetText()
		case database.FieldTextArea:
			fields[s.Label] = item.(*tview.TextArea).GetText()
		case database.FieldDropDown:
			_, val := item.(*tview.DropDown).GetCurrentOption()
			fields[s.Label] = val
		}
	}
	return fields
}

func (cf *ConnectionForm) collectOptions() (config.SQLOptions, error) {
	opts := config.SQLOptions{}

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
	modal.ShowError(cf.App.Pages, fmt.Sprintf("Failed to %s connection", action), err)
}

func (cf *ConnectionForm) cancel() {
	if cf.onCancel != nil {
		cf.onCancel()
	}
}
