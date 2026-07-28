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

const ServerInfoModalId = "ServerInfo"

type ServerInfoModal struct {
	*core.BaseElement
	*core.Flex

	content   *core.TextView
	refreshFn func()
}

func NewServerInfoModal() *ServerInfoModal {
	s := &ServerInfoModal{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		content:     core.NewTextView(),
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

	s.Flex.AddItem(s.content, 0, 1, true)
}

func (s *ServerInfoModal) setStyle() {
	s.Flex.SetStyle(s.App.GetStyles())
	s.content.SetStyle(s.App.GetStyles())
}

func (s *ServerInfoModal) setKeybindings() {
	keys := s.App.GetKeys()
	s.content.SetInputCapture(keys.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case keys.Match(keys.Common.Close, event):
			s.App.Pages.RemoveModalPage(ServerInfoModalId)
			return nil
		case keys.Match(keys.Common.Refresh, event):
			if s.refreshFn != nil {
				s.refreshFn()
			}
			return nil
		}
		return event
	}))
}

func (s *ServerInfoModal) handleEvents() {
	go s.HandleEvents(ServerInfoModalId, func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			s.setStyle()
		}
	})
}

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

	s.App.Pages.ShowModal(ServerInfoModalId, wrapper, s, true, true)
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
	if info.TLS != "" {
		fmt.Fprintf(&sb, "  [%s]TLS[-]          [%s]%s[-]\n", dim, accent, info.TLS)
	} else {
		fmt.Fprintf(&sb, "  [%s]TLS[-]          [%s]none[-]\n", dim, dim)
	}
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
