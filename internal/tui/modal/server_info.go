package modal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const ServerInfoModalId tview.Identifier = "ServerInfo"

// ServerInfoModal displays a structured dashboard of server metrics.
// It is owned by page/main.go and opened via Open().
type ServerInfoModal struct {
	*core.BaseElement
	*core.Flex

	content   *core.TextView
	hint      *tview.TextView
	refreshFn func()
}

func NewServerInfoModal() *ServerInfoModal {
	s := &ServerInfoModal{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		content:     core.NewTextView(),
		hint:        tview.NewTextView(),
	}
	s.SetIdentifier(ServerInfoModalId)
	s.SetAfterInitFunc(s.init)
	return s
}

func (s *ServerInfoModal) init() error {
	s.setLayout()
	s.setStyle()
	s.setKeybindings()
	s.handleEvents()
	return nil
}

func (s *ServerInfoModal) setLayout() {
	s.Flex.SetDirection(tview.FlexRow)
	s.Flex.SetBorder(true)
	s.Flex.SetTitle(" Server Info ")
	s.Flex.SetBorderPadding(0, 0, 1, 1)

	s.content.SetDynamicColors(true)
	s.content.SetScrollable(true)
	s.content.SetWrap(false)

	s.hint.SetDynamicColors(true)
	s.hint.SetTextAlign(tview.AlignCenter)

	s.Flex.AddItem(s.content, 0, 1, true)
	s.Flex.AddItem(s.hint, 1, 0, false)
}

func (s *ServerInfoModal) setStyle() {
	styles := s.App.GetStyles()
	s.Flex.SetStyle(styles)
	s.content.SetStyle(styles)

	bg := styles.Global.BackgroundColor.Color()
	dim := styles.Global.DimColor.String()
	text := styles.Global.SecondaryTextColor.String()

	s.hint.SetBackgroundColor(bg)
	s.hint.SetText(fmt.Sprintf(
		"[%s][[Esc][-][%s]] Close   [[%s]r[-][%s]] Refresh[-]",
		dim, dim, text, dim,
	))
}

func (s *ServerInfoModal) setKeybindings() {
	s.content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		keys := s.App.GetKeys()
		switch {
		case keys.Contains(keys.Common.Close, event.Name()):
			s.App.Pages.RemovePage(ServerInfoModalId)
			return nil
		case event.Rune() == 'r':
			if s.refreshFn != nil {
				s.refreshFn()
			}
			return nil
		}
		return event
	})
}

func (s *ServerInfoModal) handleEvents() {
	go s.HandleEvents(ServerInfoModalId, func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			s.setStyle()
		}
	})
}

// Open displays the modal with the given server info.
// If the modal is already visible (e.g. on refresh), the content is updated in place.
func (s *ServerInfoModal) Open(info *database.ServerInfo, refreshFn func()) {
	s.refreshFn = refreshFn
	s.content.SetText(s.buildContent(info))
	s.content.ScrollToBeginning()

	if s.App.Pages.HasPage(ServerInfoModalId) {
		return
	}

	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(s.Flex, 0, 4, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
		AddItem(nil, 0, 1, false)

	s.App.Pages.AddPage(ServerInfoModalId, wrapper, true, true)
	s.App.SetFocusInternal(s.content)
}

func (s *ServerInfoModal) buildContent(info *database.ServerInfo) string {
	styles := s.App.GetStyles()
	accent := styles.Global.FocusColor.String()
	label := styles.Global.SecondaryTextColor.String()
	text := styles.Global.TextColor.String()
	dim := styles.Global.DimColor.String()

	const sectionLine = 54

	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "[%s]%s:%d[-]\n\n", accent, info.Host, info.Port)

	// System Metrics
	fmt.Fprintf(&sb, "[%s]─ SYSTEM METRICS %s[-]\n", label, strings.Repeat("─", sectionLine))
	fmt.Fprintf(&sb, "  [%s]Version[-]      [%s]%s[-]\n", dim, text, info.Version)
	if info.Uptime != "" {
		fmt.Fprintf(&sb, "  [%s]Uptime[-]       [%s]%s[-]\n", dim, text, info.Uptime)
	}
	if info.ActiveSessions > 0 || info.MaxConnections > 0 {
		var connLine string
		if info.MaxConnections > 0 {
			bar := asciiProgressBar(info.ActiveSessions, info.MaxConnections, 20)
			connLine = fmt.Sprintf("[%s]%s[-] [%s]%d / %d[-]",
				accent, bar, text, info.ActiveSessions, info.MaxConnections)
		} else {
			connLine = fmt.Sprintf("[%s]%d[-]", text, info.ActiveSessions)
		}
		fmt.Fprintf(&sb, "  [%s]Connections[-]  %s\n", dim, connLine)
	}

	// Database
	fmt.Fprintf(&sb, "\n[%s]─ DATABASE %s[-]\n", label, strings.Repeat("─", sectionLine+7))
	fmt.Fprintf(&sb, "  [%s]Name[-]         [%s]%s[-]\n", dim, text, info.CurrentDB)
	if info.DatabaseSize != "" {
		fmt.Fprintf(&sb, "  [%s]Size[-]         [%s]%s[-]\n", dim, text, info.DatabaseSize)
	}
	if info.CacheHitRatio != "" {
		fmt.Fprintf(&sb, "  [%s]Cache Hit[-]    [%s]%s[-]\n", dim, text, info.CacheHitRatio)
	}

	// Driver-specific extras
	if len(info.Extra) > 0 {
		fmt.Fprintf(&sb, "\n[%s]─ EXTRA %s[-]\n", label, strings.Repeat("─", sectionLine+10))
		keys := make([]string, 0, len(info.Extra))
		for k := range info.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "  [%s]%-20s[-] [%s]%s[-]\n", dim, k, text, info.Extra[k])
		}
	}

	return sb.String()
}

// asciiProgressBar renders "[####------]" with the given fill ratio.
func asciiProgressBar(current, max int64, width int) string {
	if max <= 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	filled := int(float64(current) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + "]"
}
