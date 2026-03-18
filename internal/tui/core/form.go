package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
)

type Form struct {
	*tview.Form
}

func NewForm() *Form {
	f := &Form{Form: tview.NewForm()}
	f.Form.SetInputCapture(formNavCapture(nil))
	return f
}

// SetInputCapture wraps the provided capture function so that Ctrl+J/K always
// translate to Tab/Backtab for form field navigation, regardless of what
// page-specific bindings are layered on top.
func (f *Form) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Box {
	return f.Form.SetInputCapture(formNavCapture(capture))
}

// ApplyDropdownNavKeys wires the configured dropdown navigation keys onto every
// tview.DropDown item currently in the form. Call this after the form is fully
// populated (or re-populated after a Clear).
func (f *Form) ApplyDropdownNavKeys(k *config.KeyBindings) {
	for i := 0; i < f.GetFormItemCount(); i++ {
		if dd, ok := f.GetFormItem(i).(*tview.DropDown); ok {
			dd.SetInputCapture(dropdownInputCapture(k, dd.GetInputCapture()))
		}
	}
}

// formNavCapture prepends Ctrl+J/K → Tab/Backtab translation before any
// caller-supplied capture function.
func formNavCapture(next func(event *tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlJ:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		case tcell.KeyCtrlK:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift)
		}
		if next != nil {
			return next(event)
		}
		return event
	}
}

func (f *Form) SetStyle(style *config.Styles) {
	SetCommonStyle(f.Form, style)
	f.SetButtonBackgroundColor(style.Others.ButtonsBackgroundColor.Color())
	f.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}

// InsertFormItem inserts a form item at the given position, preserving buttons.
func (f *Form) InsertFormItem(pos int, item tview.FormItem) *Form {
	count := f.GetFormItemCount()
	if pos < 0 || pos > count {
		pos = count
	}

	existingItems := make([]tview.FormItem, count)
	for i := 0; i < count; i++ {
		existingItems[i] = f.GetFormItem(i)
	}

	f.Clear(false)
	for i := 0; i < pos; i++ {
		f.AddFormItem(existingItems[i])
	}
	f.AddFormItem(item)
	for i := pos; i < count; i++ {
		f.AddFormItem(existingItems[i])
	}

	return f
}
