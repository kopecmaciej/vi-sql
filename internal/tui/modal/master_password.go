package modal

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/security"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const MasterPasswordModalId = "MasterPasswordModal"

type MasterPasswordMode int

const (
	MasterModeSetup MasterPasswordMode = iota
	MasterModeUnlock
	MasterModeChange
)

type MasterPasswordModal struct {
	*core.BaseElement
	Form *core.Form

	mode    MasterPasswordMode
	cfg     *config.Config
	onDone  func(err error)
	onClose func()

	statusView *tview.TextView
	busy       bool
}

func NewMasterPasswordModal(mode MasterPasswordMode) *MasterPasswordModal {
	m := &MasterPasswordModal{
		BaseElement: core.NewBaseElement(),
		Form:        core.NewForm(),
		mode:        mode,
		statusView:  tview.NewTextView().SetDynamicColors(true),
	}
	m.SetAfterInitFunc(m.init)
	return m
}

func (m *MasterPasswordModal) init() error {
	m.cfg = m.App.GetConfig()
	m.setLayout()
	m.setStyle()
	m.setKeybindings()
	m.handleEvents()
	return nil
}

func (m *MasterPasswordModal) setKeybindings() {
	k := m.App.GetKeys()
	m.Form.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.FocusDown, event):
			m.Form.SetFocus(m.adjacentFormIndex(1))
			return nil
		case k.Match(k.Navigation.FocusUp, event):
			m.Form.SetFocus(m.adjacentFormIndex(-1))
			return nil
		case event.Key() == tcell.KeyEnter && !m.busy:
			focusedIdx, _ := m.Form.GetFocusedItemIndex()
			if focusedIdx >= 0 && focusedIdx < m.Form.GetFormItemCount() {
				m.submit()
				return nil
			}
		}
		return event
	}))
}

func (m *MasterPasswordModal) adjacentFormIndex(delta int) int {
	itemIdx, btnIdx := m.Form.GetFocusedItemIndex()
	total := m.Form.GetFormItemCount() + m.Form.GetButtonCount()
	cur := 0
	switch {
	case itemIdx >= 0:
		cur = itemIdx
	case btnIdx >= 0:
		cur = m.Form.GetFormItemCount() + btnIdx
	}
	return (cur + delta + total) % total
}

func (m *MasterPasswordModal) setLayout() {
	m.Form.SetBorder(true)
	m.Form.SetTitleAlign(tview.AlignCenter)
	switch m.mode {
	case MasterModeSetup:
		m.Form.SetTitle(" Set master password ")
	case MasterModeUnlock:
		m.Form.SetTitle(" Unlock with master password ")
	case MasterModeChange:
		m.Form.SetTitle(" Change master password ")
	}
	m.Form.SetCancelFunc(m.cancel)
}

func (m *MasterPasswordModal) setStyle() {
	styles := m.App.GetStyles()
	m.Form.SetStyle(styles)
	m.Form.SetFieldTextColor(styles.Global.TextColor.Color())
	m.Form.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	m.Form.SetLabelColor(styles.Global.SecondaryTextColor.Color())
	m.statusView.SetBackgroundColor(styles.Global.BackgroundColor.Color())
	m.statusView.SetTextColor(styles.Global.SecondaryTextColor.Color())
}

func (m *MasterPasswordModal) handleEvents() {
	go m.HandleEvents(MasterPasswordModalId, func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			m.App.QueueUpdateDraw(m.setStyle)
		}
	})
}

func (m *MasterPasswordModal) SetOnDone(fn func(err error)) {
	m.onDone = fn
}

func (m *MasterPasswordModal) SetOnClose(fn func()) {
	m.onClose = fn
}

func (m *MasterPasswordModal) Render() {
	m.Form.Clear(true)

	switch m.mode {
	case MasterModeSetup:
		m.Form.AddPasswordField("New password", "", 0, '*', nil)
		m.Form.AddPasswordField("Confirm", "", 0, '*', nil)
	case MasterModeUnlock:
		m.Form.AddPasswordField("Password", "", 0, '*', nil)
	case MasterModeChange:
		m.Form.AddPasswordField("Current password", "", 0, '*', nil)
		m.Form.AddPasswordField("New password", "", 0, '*', nil)
		m.Form.AddPasswordField("Confirm new", "", 0, '*', nil)
	}

	m.Form.AddFormItem(m.statusView)

	var submitLabel string
	switch m.mode {
	case MasterModeUnlock:
		submitLabel = "Unlock"
	case MasterModeChange:
		submitLabel = "Change"
	default:
		submitLabel = "Set"
	}
	m.Form.AddButton(submitLabel, m.submit)
	if m.mode != MasterModeUnlock {
		m.Form.AddButton("Cancel", m.cancel)
	}

	m.Form.ApplyClipboard()
	m.App.Pages.AddModalPage(MasterPasswordModalId, core.CenteredFlex(m.Form, 3, 6), true, true)
	m.App.SetFocusOnly(m.Form)
}

func (m *MasterPasswordModal) Hide() {
	if m.App.Pages.HasPage(MasterPasswordModalId) {
		m.App.Pages.RemoveModalPage(MasterPasswordModalId)
	}
	if m.onClose != nil {
		m.onClose()
	}
}

func (m *MasterPasswordModal) cancel() {
	m.Hide()
	if m.onDone != nil {
		m.onDone(fmt.Errorf("cancelled"))
	}
}

func (m *MasterPasswordModal) setStatus(text string) {
	m.statusView.SetText(text)
}

func (m *MasterPasswordModal) setBusy(busy bool) {
	m.busy = busy
	for i := 0; i < m.Form.GetButtonCount(); i++ {
		m.Form.GetButton(i).SetDisabled(busy)
	}
}

func (m *MasterPasswordModal) submit() {
	if m.busy {
		return
	}
	switch m.mode {
	case MasterModeSetup:
		m.runSetup()
	case MasterModeUnlock:
		m.runUnlock()
	case MasterModeChange:
		m.runChange()
	}
}

func (m *MasterPasswordModal) passwordFieldText(label string) string {
	item := m.Form.GetFormItemByLabel(label)
	if item == nil {
		return ""
	}
	if f, ok := item.(*tview.InputField); ok {
		return f.GetText()
	}
	return ""
}

func (m *MasterPasswordModal) runSetup() {
	pass := m.passwordFieldText("New password")
	confirm := m.passwordFieldText("Confirm")
	if pass == "" {
		m.setStatus("[red]Password must not be empty[-]")
		return
	}
	if pass != confirm {
		m.setStatus("[red]Passwords do not match[-]")
		return
	}

	salt, err := security.GenerateSalt()
	if err != nil {
		m.setStatus("[red]" + err.Error() + "[-]")
		return
	}
	params := m.cfg.MasterParams()

	m.setBusy(true)
	m.setStatus("Deriving key…")
	ch := security.DeriveKeyAsync(pass, salt, params)
	go func() {
		res := <-ch
		m.App.QueueUpdateDraw(func() {
			m.setBusy(false)
			if res.Err != nil {
				m.setStatus("[red]" + res.Err.Error() + "[-]")
				return
			}
			if err := m.cfg.ApplyMasterSetup(res.Key, salt, params); err != nil {
				m.setStatus("[red]" + err.Error() + "[-]")
				return
			}
			m.finish(nil)
		})
	}()
}

// Note: unlock has no attempt cap — the user can keep guessing. Brute-forcing
// the passphrase is gated by Argon2id (~100-300ms per try with default params),
// not by an in-app counter. Add a counter here if local rate-limiting becomes
// a concern.
func (m *MasterPasswordModal) runUnlock() {
	pass := m.passwordFieldText("Password")
	if pass == "" {
		m.setStatus("[red]Password must not be empty[-]")
		return
	}
	params := m.cfg.MasterParams()
	salt := m.cfg.Security.MasterSalt

	m.setBusy(true)
	m.setStatus("Deriving key…")
	ch := security.DeriveKeyAsync(pass, salt, params)
	go func() {
		res := <-ch
		m.App.QueueUpdateDraw(func() {
			m.setBusy(false)
			if res.Err != nil {
				m.setStatus("[red]" + res.Err.Error() + "[-]")
				return
			}
			if err := m.cfg.ApplyMasterUnlock(res.Key); err != nil {
				m.setStatus("[red]Wrong password — try again[-]")
				m.clearField("Password")
				m.App.SetFocusOnly(m.Form)
				return
			}
			m.finish(nil)
		})
	}()
}

func (m *MasterPasswordModal) runChange() {
	current := m.passwordFieldText("Current password")
	newPass := m.passwordFieldText("New password")
	confirm := m.passwordFieldText("Confirm new")
	if current == "" || newPass == "" {
		m.setStatus("[red]Passwords must not be empty[-]")
		return
	}
	if newPass != confirm {
		m.setStatus("[red]New passwords do not match[-]")
		return
	}

	params := m.cfg.MasterParams()
	currentSalt := m.cfg.Security.MasterSalt

	m.setBusy(true)
	m.setStatus("Verifying current password…")
	chCurrent := security.DeriveKeyAsync(current, currentSalt, params)
	go func() {
		resCurrent := <-chCurrent
		if resCurrent.Err != nil {
			m.App.QueueUpdateDraw(func() {
				m.setBusy(false)
				m.setStatus("[red]" + resCurrent.Err.Error() + "[-]")
			})
			return
		}
		if err := m.cfg.ApplyMasterUnlock(resCurrent.Key); err != nil {
			m.App.QueueUpdateDraw(func() {
				m.setBusy(false)
				m.setStatus("[red]Current password is incorrect[-]")
				m.clearField("Current password")
				m.App.SetFocusOnly(m.Form)
			})
			return
		}

		newSalt, err := security.GenerateSalt()
		if err != nil {
			m.App.QueueUpdateDraw(func() {
				m.setBusy(false)
				m.setStatus("[red]" + err.Error() + "[-]")
			})
			return
		}
		m.App.QueueUpdateDraw(func() {
			m.setStatus("Deriving new key…")
		})
		chNew := security.DeriveKeyAsync(newPass, newSalt, params)
		resNew := <-chNew
		m.App.QueueUpdateDraw(func() {
			m.setBusy(false)
			if resNew.Err != nil {
				m.setStatus("[red]" + resNew.Err.Error() + "[-]")
				return
			}
			if err := m.cfg.ApplyMasterChange(resNew.Key, newSalt, params); err != nil {
				m.setStatus("[red]" + err.Error() + "[-]")
				return
			}
			m.finish(nil)
		})
	}()
}

func (m *MasterPasswordModal) clearField(label string) {
	if item := m.Form.GetFormItemByLabel(label); item != nil {
		if f, ok := item.(*tview.InputField); ok {
			f.SetText("")
		}
	}
}

func (m *MasterPasswordModal) finish(err error) {
	m.Hide()
	if m.onDone != nil {
		m.onDone(err)
	}
}
