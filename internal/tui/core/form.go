package core

import (
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
)

type Form struct {
	*tview.Form
}

func NewForm() *Form {
	return &Form{
		Form: tview.NewForm(),
	}
}

func (f *Form) SetStyle(style *config.Styles) {
	SetCommonStyle(f.Form, style)
	f.SetButtonBackgroundColor(style.Others.ButtonsBackgroundColor.Color())
	f.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}

// InsertFormItem inserts a form item at the given position.
// Buttons are not preserved — re-add them separately if needed.
func (f *Form) InsertFormItem(pos int, item tview.FormItem) *Form {
	count := f.GetFormItemCount()
	if pos < 0 || pos > count {
		pos = count
	}

	existingItems := make([]tview.FormItem, count)
	for i := 0; i < count; i++ {
		existingItems[i] = f.GetFormItem(i)
	}

	f.Clear(true)
	for i := 0; i < pos; i++ {
		f.AddFormItem(existingItems[i])
	}
	f.AddFormItem(item)
	for i := pos; i < count; i++ {
		f.AddFormItem(existingItems[i])
	}

	return f
}
