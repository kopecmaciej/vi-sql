package primitives

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
)

// InputModal is a simple input field displayed as a centered modal.
type InputModal struct {
	*tview.Box

	input *tview.InputField
	label string
}

func NewInputModal() *InputModal {
	return &InputModal{
		Box:   tview.NewBox(),
		input: tview.NewInputField(),
	}
}

func (mi *InputModal) Draw(screen tcell.Screen) {
	screenWidth, screenHeight := screen.Size()

	minWidth, minHeight := 50, 6
	width, height := screenWidth/5, screenHeight/6
	if width < minWidth {
		width = minWidth
	}
	if height < minHeight {
		height = minHeight
	}

	x, y := (screenWidth-width)/2, (screenHeight-height)/2

	mi.Box.SetRect(x, y, width, height)
	mi.Box.DrawForSubclass(screen, mi.input)

	inputX, inputY, inputWidth, _ := mi.GetInnerRect()
	tview.Print(screen, mi.label, inputX, inputY, inputWidth, tview.AlignCenter, tcell.ColorYellow)

	inputY += 2
	inputX += 2
	inputWidth -= 4
	mi.input.SetRect(inputX, inputY, inputWidth, 1)
	mi.input.Draw(screen)
}

func (mi *InputModal) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return mi.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		mi.input.InputHandler()(event, setFocus)
	})
}

func (mi *InputModal) SetText(text string) *InputModal {
	mi.input.SetText(text)
	return mi
}

func (mi *InputModal) GetText() string {
	return mi.input.GetText()
}

func (mi *InputModal) SetLabel(label string) *InputModal {
	mi.label = label
	return mi
}

func (mi *InputModal) SetInputLabel(label string) *InputModal {
	mi.input.SetLabel(label)
	return mi
}

func (mi *InputModal) SetLabelColor(color tcell.Color) *InputModal {
	mi.input.SetLabelColor(color)
	return mi
}

func (mi *InputModal) SetFieldBackgroundColor(color tcell.Color) *InputModal {
	mi.input.SetFieldBackgroundColor(color)
	return mi
}

func (mi *InputModal) SetFieldTextColor(color tcell.Color) *InputModal {
	mi.input.SetFieldTextColor(color)
	return mi
}

func (mi *InputModal) SetBackgroundColor(color tcell.Color) *InputModal {
	mi.Box.SetBackgroundColor(color)
	return mi
}

func (mi *InputModal) SetBorderColor(color tcell.Color) *InputModal {
	mi.Box.SetBorderColor(color)
	return mi
}
