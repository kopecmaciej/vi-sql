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
	return &Form{Form: tview.NewForm()}
}

// ApplyFormNavKeys installs an input capture on the form that translates the
// configured FocusDown/FocusUp keys to Tab/Backtab. Raw Tab/Backtab are blocked
// so only the configured keys navigate between fields. Call this after any
// page-specific SetInputCapture so the translation wraps the inner handler.
func (f *Form) ApplyFormNavKeys(k *config.KeyBindings) {
	existing := f.Form.GetInputCapture()
	f.Form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Navigation.FocusDown, event.Name()):
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		case k.Contains(k.Navigation.FocusUp, event.Name()):
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift)
		case event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab:
			return nil
		}
		if existing != nil {
			return existing(event)
		}
		return event
	})
}

// ApplyDropdownNavKeys wires the configured dropdown navigation keys onto every
// tview.DropDown item currently in the form. Call this after the form is fully
// populated (or re-populated after a Clear).
func (f *Form) ApplyDropdownNavKeys(k *config.KeyBindings) {
	for i := 0; i < f.GetFormItemCount(); i++ {
		if dd, ok := f.GetFormItem(i).(*tview.DropDown); ok {
			dd.SetInputCapture(DropdownInputCapture(k, dd.GetInputCapture()))
		}
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
