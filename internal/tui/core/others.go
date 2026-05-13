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
		rawInput bool
	}
	TextArea struct {
		*tview.TextArea
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

// CenteredFlex wraps inner in a 1:widthRatio:1 / 1:heightRatio:1 flex grid,
// producing a centered modal effect when added as a full-screen page.
func CenteredFlex(inner tview.Primitive, widthRatio, heightRatio int) *tview.Flex {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(inner, 0, heightRatio, true).
			AddItem(nil, 0, 1, false), 0, widthRatio, true).
		AddItem(nil, 0, 1, false)
}

func NewList() *List {
	return &List{List: tview.NewList()}
}

func NewTextView() *TextView {
	return &TextView{TextView: tview.NewTextView()}
}

// SetRawInput marks this TextView as a raw-input surface (e.g. a key-capture
// panel) so the sequence machine treats it like a text input and does not
// absorb sequence prefixes while it has focus.
func (t *TextView) SetRawInput(v bool) { t.rawInput = v }
func (t *TextView) IsRawInput() bool   { return t.rawInput }

func NewTextArea() *TextArea {
	return &TextArea{TextArea: tview.NewTextArea()}
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
	bg := style.Global.BackgroundColor.Color()
	fg := style.Global.TextColor.Color()
	l.SetMainTextStyle(tcell.StyleDefault.Background(bg).Foreground(fg))
	l.SetSecondaryTextStyle(tcell.StyleDefault.Background(bg).Foreground(fg))
}

func (t *TextView) SetStyle(style *config.Styles) {
	SetCommonStyle(t.TextView, style)
	t.SetTextColor(style.Global.TextColor.Color())
}

func (t *TextArea) SetStyle(style *config.Styles) {
	SetCommonStyle(t.TextArea, style)
	t.TextArea.SetTextStyle(tcell.StyleDefault.
		Background(style.Global.BackgroundColor.Color()).
		Foreground(style.Global.TextColor.Color()))
}

func (t *TreeView) SetStyle(style *config.Styles) {
	SetCommonStyle(t.TreeView, style)
}

func (i *InputField) SetStyle(style *config.Styles) {
	SetCommonStyle(i.InputField, style)
	i.SetLabelStyle(tcell.StyleDefault.
		Foreground(style.Global.TextColor.Color()).
		Background(style.Global.BackgroundColor.Color()))
	i.SetFieldBackgroundColor(style.Global.ContrastBackgroundColor.Color())
	i.SetFieldTextColor(style.Global.TextColor.Color())
	i.SetPlaceholderTextColor(style.Global.TextColor.Color())
}

func (m *Modal) SetStyle(style *config.Styles) {
	SetCommonStyle(m.Box, style)
	m.SetBackgroundColor(style.Global.BackgroundColor.Color())
	m.SetTextColor(style.Global.TextColor.Color())
	m.SetButtonBackgroundColor(style.Global.MoreContrastBackgroundColor.Color())
	m.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}

func (v *ViewModal) SetStyle(style *config.Styles) {
	SetCommonStyle(v.ViewModal, style)
	v.SetButtonBackgroundColor(style.Global.MoreContrastBackgroundColor.Color())
	v.SetButtonTextColor(style.Others.ButtonsTextColor.Color())
}

// DropdownInputCapture returns an input capture that translates configured
// dropdown keys to the arrow/enter keys that tview's autocomplete list understands.
// Use this on any InputField with autocomplete or tview.DropDown.
func DropdownInputCapture(k *config.KeyBindings, next func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.AutocompleteUp, event):
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case k.Match(k.Navigation.AutocompleteDown, event):
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case k.Match(k.Navigation.AutocompleteAccept, event):
			return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		}
		if next != nil {
			return next(event)
		}
		return event
	})
}
