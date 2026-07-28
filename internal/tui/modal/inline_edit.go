package modal

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const InlineEditModalId = "InlineEditModal"

// InlineEditModal is a centered form-based modal for editing a single cell value.
// Short values use an InputField; long or multi-line values use a TextArea.
type InlineEditModal struct {
	*core.BaseElement
	Form *core.Form

	fieldName      string
	applyCallback  func(fieldName, newValue string) error
	cancelCallback func()
}

func NewInlineEditModal() *InlineEditModal {
	iem := &InlineEditModal{
		BaseElement: core.NewBaseElement(),
		Form:        core.NewForm(),
	}
	iem.SetAfterInitFunc(iem.init)
	return iem
}

func (iem *InlineEditModal) init() error {
	iem.setLayout()
	iem.setStyle()
	iem.setKeybindings()
	iem.handleEvents()
	return nil
}

func (iem *InlineEditModal) setLayout() {
	iem.Form.SetIdentifier(InlineEditModalId)
	iem.Form.SetBorder(true)
	iem.Form.SetTitle(" Inline Edit ")
	iem.Form.SetTitleAlign(tview.AlignCenter)
	iem.Form.SetBorderPadding(2, 2, 2, 2)
}

func (iem *InlineEditModal) setStyle() {
	styles := iem.App.GetStyles()
	iem.Form.SetStyle(styles)
	fieldBg := styles.Global.ContrastBackgroundColor.Color()
	fieldFg := styles.Global.TextColor.Color()
	iem.Form.SetFieldTextColor(fieldFg)
	iem.Form.SetFieldBackgroundColor(fieldBg)
	iem.Form.SetLabelColor(styles.Global.SecondaryTextColor.Color())

	if iem.Form.GetFormItemCount() > 0 {
		switch item := iem.Form.GetFormItem(0).(type) {
		case *tview.InputField:
			item.SetFieldBackgroundColor(fieldBg)
			item.SetFieldTextColor(fieldFg)
		case *tview.TextArea:
			item.SetTextStyle(tcell.StyleDefault.Background(fieldBg).Foreground(fieldFg))
		}
	}
}

func (iem *InlineEditModal) setKeybindings() {
	k := iem.App.GetKeys()
	iem.Form.SetCancelFunc(func() {
		if iem.cancelCallback != nil {
			iem.cancelCallback()
		}
	})
	iem.Form.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Common.Confirm, event):
			iem.handleApply()
			return nil
		case event.Key() == tcell.KeyEnter:
			// Enter confirms on single-line InputField; TextArea needs Confirm key.
			if item := iem.Form.GetFormItem(0); item != nil {
				if _, ok := item.(*tview.InputField); ok {
					iem.handleApply()
					return nil
				}
			}
		}
		return event
	}))
}

func (iem *InlineEditModal) handleEvents() {
	go iem.HandleEvents(InlineEditModalId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			iem.setStyle()
		}
	})
}

// SetApplyCallback sets the function called when the user confirms the edit.
// Returning a non-nil error keeps the modal open and shows an error.
func (iem *InlineEditModal) SetApplyCallback(callback func(fieldName, newValue string) error) {
	iem.applyCallback = callback
}

// SetCancelCallback sets the function called when the user cancels.
func (iem *InlineEditModal) SetCancelCallback(callback func()) {
	iem.cancelCallback = callback
}

func (iem *InlineEditModal) handleApply() {
	if iem.Form.GetFormItemCount() == 0 {
		return
	}

	var newValue string
	switch field := iem.Form.GetFormItem(0).(type) {
	case *tview.InputField:
		newValue = field.GetText()
	case *tview.TextArea:
		newValue = field.GetText()
	default:
		return
	}

	if iem.applyCallback != nil {
		if err := iem.applyCallback(iem.fieldName, newValue); err != nil {
			ShowError(iem.App.Pages, "Error applying edit", err)
		}
	}
}

// Render populates the form with the given field name and current value,
// then shows the modal. Short values use an InputField; longer ones a TextArea.
func (iem *InlineEditModal) Render(fieldName, currentValue string) {
	iem.Form.Clear(true)
	iem.fieldName = fieldName

	styles := iem.App.GetStyles()
	fieldBg := styles.Global.ContrastBackgroundColor.Color()
	fieldFg := styles.Global.TextColor.Color()

	if len(currentValue) > 100 {
		ta := tview.NewTextArea().
			SetText(currentValue, true).
			SetWrap(true).
			SetSize(8, 0)
		ta.SetTextStyle(tcell.StyleDefault.Background(fieldBg).Foreground(fieldFg))
		iem.Form.AddFormItem(ta)
	} else {
		iem.Form.AddFormItem(
			tview.NewInputField().
				SetText(currentValue).
				SetFieldWidth(0).
				SetFieldBackgroundColor(fieldBg).
				SetFieldTextColor(fieldFg),
		)
	}

	iem.Form.ApplyClipboard()
	iem.Show()
}

func (iem *InlineEditModal) Show() {
	iem.App.Pages.ShowModal(InlineEditModalId, core.CenteredFlex(iem.Form, 1, 1), iem.Form, true, true)
}

func (iem *InlineEditModal) Hide() {
	iem.App.Pages.RemoveModalPage(InlineEditModalId)
}
