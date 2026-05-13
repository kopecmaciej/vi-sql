package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
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
	f.Form.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.FocusDown, event):
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		case k.Match(k.Navigation.FocusUp, event):
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModShift)
		case event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab:
			return nil
		}
		if existing != nil {
			return existing(event)
		}
		return event
	}))
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

// ApplyClipboard wires the system clipboard onto every InputField and TextArea
// currently in the form. Call this after the form is fully populated.
func (f *Form) ApplyClipboard() {
	copy, paste := util.GetClipboard()
	for i := 0; i < f.GetFormItemCount(); i++ {
		switch item := f.GetFormItem(i).(type) {
		case *tview.InputField:
			item.SetClipboard(copy, paste)
		case *tview.TextArea:
			item.SetClipboard(copy, paste)
		}
	}
}

func (f *Form) SetStyle(style *config.Styles) {
	SetCommonStyle(f.Form, style)
	f.SetButtonBackgroundColor(style.Global.MoreContrastBackgroundColor.Color())
	f.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}

// FormGroup is a set of form items that can be shown or hidden as a unit.
type FormGroup struct {
	visible bool
	build   func() []tview.FormItem
}

func NewFormGroup(visible bool, build func() []tview.FormItem) *FormGroup {
	return &FormGroup{visible: visible, build: build}
}

func (g *FormGroup) IsVisible() bool {
	return g.visible
}

// SetVisible updates the visibility flag without touching the form.
// Call form.RenderGroups after to re-render in the correct order.
func (g *FormGroup) SetVisible(visible bool) {
	g.visible = visible
}

func (g *FormGroup) Show(f *Form) {
	if g.visible {
		return
	}
	g.visible = true
	for _, item := range g.build() {
		f.AddFormItem(item)
	}
}

func (g *FormGroup) Hide(f *Form) {
	if !g.visible {
		return
	}
	g.visible = false
	for _, item := range g.build() {
		if idx := f.GetFormItemIndex(item.GetLabel()); idx >= 0 {
			f.RemoveFormItem(idx)
		}
	}
}

// RenderGroups clears the form and re-adds items from all visible groups.
// Call ApplyDropdownNavKeys afterward if the form contains dropdowns.
func (f *Form) RenderGroups(groups []*FormGroup) {
	f.Clear(false)
	for _, g := range groups {
		if g.visible {
			for _, item := range g.build() {
				f.AddFormItem(item)
			}
		}
	}
}

// InsertFormItem inserts a form item at the given position, preserving buttons.
func (f *Form) InsertFormItem(pos int, item tview.FormItem) *Form {
	count := f.GetFormItemCount()
	if pos < 0 || pos > count {
		pos = count
	}

	existingItems := make([]tview.FormItem, count)
	for i := range count {
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
