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
	*core.FormModal

	fieldName      string
	applyCallback  func(fieldName, newValue string) error
	cancelCallback func()
}

func NewInlineEditModal() *InlineEditModal {
	iem := &InlineEditModal{
		BaseElement: core.NewBaseElement(),
		FormModal:   core.NewFormModal(),
	}
	iem.SetIdentifier(InlineEditModalId)
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
	iem.SetTitle(" Inline Edit ")
	iem.SetBorder(true)
	iem.SetTitleAlign(tview.AlignCenter)
	iem.Form.SetBorderPadding(2, 2, 2, 2)
}

func (iem *InlineEditModal) setStyle() {
	styles := iem.App.GetStyles()
	iem.SetStyle(styles)
	iem.Form.SetFieldTextColor(styles.Global.TextColor.Color())
	iem.Form.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	iem.Form.SetLabelColor(styles.Global.SecondaryTextColor.Color())
}

func (iem *InlineEditModal) setKeybindings() {
	iem.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if iem.cancelCallback != nil {
				iem.cancelCallback()
			}
			return nil
		case tcell.KeyEnter:
			focusedIdx, _ := iem.Form.GetFocusedItemIndex()
			if focusedIdx == 0 {
				iem.handleApply()
				return nil
			}
		}
		return event
	})
}

func (iem *InlineEditModal) handleEvents() {
	go iem.HandleEvents(iem.GetIdentifier(), func(event manager.EventMsg) {
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

	if len(currentValue) > 100 {
		iem.Form.AddFormItem(
			tview.NewTextArea().
				SetText(currentValue, true).
				SetWrap(true).
				SetSize(8, 0),
		)
	} else {
		iem.Form.AddFormItem(
			tview.NewInputField().
				SetText(currentValue).
				SetFieldWidth(0),
		)
	}

	iem.Form.AddButton("Apply", func() { iem.handleApply() })
	iem.Form.AddButton("Cancel", func() {
		if iem.cancelCallback != nil {
			iem.cancelCallback()
		}
	})

	iem.Show()
}

func (iem *InlineEditModal) Show() {
	iem.App.Pages.AddPage(InlineEditModalId, iem, true, true)
}

func (iem *InlineEditModal) Hide() {
	iem.App.Pages.RemovePage(InlineEditModalId)
}
