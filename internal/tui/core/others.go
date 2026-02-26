package core

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
)

// Styler is an interface for components that can be styled.
type Styler interface {
	SetBackgroundColor(tcell.Color) *tview.Box
	SetBorderColor(tcell.Color) *tview.Box
	SetTitleColor(tcell.Color) *tview.Box
	SetFocusStyle(tcell.Style) *tview.Box
}

// SetCommonStyle applies common styling to any component implementing the Styler interface.
func SetCommonStyle(s Styler, style *config.Styles) {
	s.SetBackgroundColor(style.Global.BackgroundColor.Color())
	s.SetBorderColor(style.Global.BorderColor.Color())
	s.SetTitleColor(style.Global.TitleColor.Color())
	s.SetFocusStyle(tcell.StyleDefault.
		Foreground(style.Global.FocusColor.Color()).
		Background(style.Global.BackgroundColor.Color()))
}

type (
	Flex struct {
		*tview.Flex
	}
	List struct {
		*tview.List
	}
	TextView struct {
		*tview.TextView
	}
	TreeView struct {
		*tview.TreeView
	}
	InputField struct {
		*tview.InputField
	}
	Modal struct {
		*tview.Modal
	}
	ViewModal struct {
		*primitives.ViewModal
	}
)

func NewFlex() *Flex {
	return &Flex{Flex: tview.NewFlex()}
}

func NewList() *List {
	return &List{List: tview.NewList()}
}

func NewTextView() *TextView {
	return &TextView{TextView: tview.NewTextView()}
}

func NewTreeView() *TreeView {
	return &TreeView{TreeView: tview.NewTreeView()}
}

func NewInputField() *InputField {
	return &InputField{InputField: tview.NewInputField()}
}

func NewModal() *Modal {
	return &Modal{Modal: tview.NewModal()}
}

func NewViewModal() *ViewModal {
	return &ViewModal{ViewModal: primitives.NewViewModal()}
}

func (f *Flex) SetStyle(style *config.Styles) {
	SetCommonStyle(f.Flex, style)
}

func (l *List) SetStyle(style *config.Styles) {
	SetCommonStyle(l.List, style)
}

func (t *TextView) SetStyle(style *config.Styles) {
	SetCommonStyle(t.TextView, style)
	t.SetTextColor(style.Global.TextColor.Color())
}

func (t *TreeView) SetStyle(style *config.Styles) {
	SetCommonStyle(t.TreeView, style)
}

func (i *InputField) SetStyle(style *config.Styles) {
	SetCommonStyle(i.InputField, style)
	i.SetLabelStyle(tcell.StyleDefault.
		Foreground(style.Global.TextColor.Color()).
		Background(style.Global.BackgroundColor.Color()))
	i.SetFieldBackgroundColor(style.Global.BackgroundColor.Color())
	i.SetFieldTextColor(style.Global.TextColor.Color())
	i.SetPlaceholderTextColor(style.Global.TextColor.Color())
}

func (m *Modal) SetStyle(style *config.Styles) {
	SetCommonStyle(m.Box, style)
	m.SetBackgroundColor(style.Global.BackgroundColor.Color())
	m.SetTextColor(style.Global.TextColor.Color())
	m.SetButtonBackgroundColor(style.Others.ButtonsBackgroundColor.Color())
	m.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}

func (v *ViewModal) SetStyle(style *config.Styles) {
	SetCommonStyle(v.ViewModal, style)
	v.SetButtonBackgroundColor(style.Others.ButtonsBackgroundColor.Color())
	v.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}
