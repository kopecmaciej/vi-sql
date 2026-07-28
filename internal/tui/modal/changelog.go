package modal

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/assets"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"gopkg.in/yaml.v3"
)

const ChangelogModalId = "Changelog"

// MigrationRegistry maps migration keys (from changelog.yaml "migration" field) to their
// implementation functions. Register entries here when a release requires a one-time data
// migration on upgrade. Currently empty — reserved for future use.
var MigrationRegistry = map[string]func() error{}

type changelogYAML struct {
	Entries []changelogEntryYAML `yaml:"entries"`
}

type changelogEntryYAML struct {
	Version   string   `yaml:"version"`
	Breaking  bool     `yaml:"breaking"`
	Title     string   `yaml:"title"`
	Changes   []string `yaml:"changes"`
	Migration string   `yaml:"migration"`
}

func LoadChangelog() []util.ChangelogEntry {
	var raw changelogYAML
	if err := yaml.Unmarshal(assets.Changelog, &raw); err != nil {
		return nil
	}

	entries := make([]util.ChangelogEntry, 0, len(raw.Entries))
	for _, e := range raw.Entries {
		entry := util.ChangelogEntry{
			Version:  e.Version,
			Breaking: e.Breaking,
			Title:    e.Title,
			Changes:  e.Changes,
		}
		if fn, ok := MigrationRegistry[e.Migration]; ok {
			entry.MigrationFn = fn
		}
		entries = append(entries, entry)
	}
	return entries
}

// Changelog is a startup modal showing release notes.
type Changelog struct {
	*core.BaseElement
	*core.Flex

	textView  *core.TextView
	form      *core.Form
	lastWidth int

	entries     []util.ChangelogEntry
	onProceed   func()
	onQuit      func()
	hasBreaking bool
}

func NewChangelog(entries []util.ChangelogEntry) *Changelog {
	hasBreaking := false
	for _, e := range entries {
		if e.Breaking {
			hasBreaking = true
			break
		}
	}

	c := &Changelog{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		textView:    core.NewTextView(),
		form:        core.NewForm(),
		entries:     entries,
		hasBreaking: hasBreaking,
	}
	c.SetIdentifier(ChangelogModalId)
	c.SetAfterInitFunc(c.init)
	return c
}

func (c *Changelog) init() error {
	c.setLayout()
	c.setStyle()
	go c.HandleEvents(c.GetIdentifier(), func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			c.setStyle()
		}
	})
	return nil
}

func (c *Changelog) setLayout() {
	c.Flex.SetDirection(tview.FlexRow)
	c.Flex.SetBorder(true)
	c.Flex.SetTitle(" Release Notes ")

	c.textView.SetDynamicColors(true).SetScrollable(true).SetWrap(true)

	c.form.SetButtonsAlign(tview.AlignCenter)
	c.form.SetBorderPadding(0, 0, 0, 0)
	c.addButtons()

	c.Flex.AddItem(c.textView, 0, 1, false)
	c.Flex.AddItem(tview.NewBox(), 1, 0, false)
	c.Flex.AddItem(c.form, 3, 0, true)
}

func (c *Changelog) addButtons() {
	if c.hasBreaking {
		c.form.AddButton("Proceed", func() {
			if c.onProceed != nil {
				c.onProceed()
			}
		})
		c.form.AddButton("Quit", func() {
			if c.onQuit != nil {
				c.onQuit()
			}
		})
	} else {
		c.form.AddButton("Continue", func() {
			if c.onProceed != nil {
				c.onProceed()
			}
		})
	}

	c.form.SetCancelFunc(func() {
		if c.hasBreaking {
			if c.onQuit != nil {
				c.onQuit()
			}
		} else {
			if c.onProceed != nil {
				c.onProceed()
			}
		}
	})
}

func (c *Changelog) setStyle() {
	styles := c.App.GetStyles()

	core.SetCommonStyle(c.Flex, styles)
	c.textView.SetStyle(styles)
	c.form.SetStyle(styles)

	if c.lastWidth > 0 {
		c.textView.SetText(c.buildText(c.lastWidth))
	}
}

func (c *Changelog) Focus(delegate func(tview.Primitive)) {
	delegate(c.form)
}

func (c *Changelog) HasFocus() bool {
	return c.form.HasFocus() || c.textView.HasFocus()
}

func (c *Changelog) Draw(screen tcell.Screen) {
	// Compute inner text width from screen size: modal is 3/5 screen width, minus Flex border (2 cols).
	screenW, _ := screen.Size()
	w := screenW*3/5 - 2
	if w > 0 && w != c.lastWidth {
		c.lastWidth = w
		c.textView.SetText(c.buildText(w))
	}
	c.Flex.Draw(screen)
}

func (c *Changelog) activateButton() {
	for i := 0; i < c.form.GetButtonCount(); i++ {
		btn := c.form.GetButton(i)
		if btn.HasFocus() {
			switch btn.GetLabel() {
			case "Continue", "Proceed":
				if c.onProceed != nil {
					c.onProceed()
				}
			case "Quit":
				if c.onQuit != nil {
					c.onQuit()
				}
			}
			return
		}
	}
	if !c.hasBreaking && c.onProceed != nil {
		c.onProceed()
	}
}

func (c *Changelog) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return c.Flex.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		k := c.App.GetKeys()
		switch {
		case event.Key() == tcell.KeyEnter:
			c.activateButton()
		case k.Match(k.Navigation.MoveDown, event):
			c.textView.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), setFocus)
		case k.Match(k.Navigation.MoveUp, event):
			c.textView.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), setFocus)
		case k.Match(k.Navigation.MoveLeft, event):
			c.form.InputHandler()(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone), setFocus)
		case k.Match(k.Navigation.MoveRight, event):
			c.form.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), setFocus)
		default:
			c.form.InputHandler()(event, setFocus)
		}
	})
}

func (c *Changelog) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return c.Flex.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		if consumed, capture = c.form.MouseHandler()(action, event, setFocus); consumed {
			return
		}
		consumed, capture = c.textView.MouseHandler()(action, event, setFocus)
		return
	})
}

func (c *Changelog) SetOnProceed(fn func()) { c.onProceed = fn }
func (c *Changelog) SetOnQuit(fn func())    { c.onQuit = fn }

func (c *Changelog) Render() {
	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(c, 0, 3, true).
			AddItem(nil, 0, 1, false), 0, 3, true).
		AddItem(nil, 0, 1, false)

	c.App.Pages.AddModalPage(ChangelogModalId, wrapper, true, true)
	c.App.SetFocusOnly(c)
}

// changeGroup holds a named set of changelog items that share the same category.
type changeGroup struct {
	key    string // matches knownCategories prefix key
	symbol string
	label  string
	items  []string
}

var knownCategories = []struct {
	prefix string
	symbol string
	label  string
}{
	{"New", "✦", "Features"},
	{"Improvement", "◈", "Improvements"},
	{"Fix", "⬡", "Bug Fixes"},
	{"Breaking", "⚑", "Breaking Changes"},
}

func groupChanges(changes []string) []changeGroup {
	indexMap := map[string]int{}
	var groups []changeGroup

	for _, ch := range changes {
		matched := false
		for _, cat := range knownCategories {
			pfx := cat.prefix + ": "
			text, found := strings.CutPrefix(ch, pfx)
			if found {
				if idx, ok := indexMap[cat.prefix]; ok {
					groups[idx].items = append(groups[idx].items, text)
				} else {
					indexMap[cat.prefix] = len(groups)
					groups = append(groups, changeGroup{
						key: cat.prefix, symbol: cat.symbol, label: cat.label, items: []string{text},
					})
				}
				matched = true
				break
			}
		}
		if !matched {
			const other = "__other__"
			if idx, ok := indexMap[other]; ok {
				groups[idx].items = append(groups[idx].items, ch)
			} else {
				indexMap[other] = len(groups)
				groups = append(groups, changeGroup{key: other, symbol: "·", label: "Other", items: []string{ch}})
			}
		}
	}
	return groups
}

func (c *Changelog) buildText(width int) string {
	styles := c.App.GetStyles()
	bg := styles.Global.BackgroundColor.String()
	fg := styles.Global.TextColor.String()
	dimColor := styles.Global.DimColor.String()
	contrastBg := styles.Global.ContrastBackgroundColor.String()
	focusColor := styles.Global.FocusColor.String()
	secondaryColor := styles.Global.SecondaryTextColor.String()
	fixColor := styles.SQLEditor.NumberColor.String() // orange

	categoryColors := map[string][2]string{
		"New":         {bg, focusColor},
		"Improvement": {bg, secondaryColor},
		"Fix":         {bg, fixColor},
		"Breaking":    {bg, "red"},
	}

	separator := fmt.Sprintf("[%s]%s[-:-:-]", dimColor, strings.Repeat("─", width))

	var sb strings.Builder

	title := "  What's new in vi-sql  "
	pad := max((width-len(title))/2, 0)
	fmt.Fprintf(&sb, "%s[%s:%s:b]%s[-:-:-]\n", strings.Repeat(" ", pad), bg, focusColor, title)
	sb.WriteString(separator + "\n\n")

	for i, e := range c.entries {
		if i > 0 {
			sb.WriteString("\n" + separator + "\n\n")
		}

		if e.Breaking {
			fmt.Fprintf(&sb, "[%s:red:b]  v%s  [-:-:-]  [%s::b]%s[::-]  [red]⚠ breaking[-]\n\n",
				bg, e.Version, fg, e.Title)
		} else {
			fmt.Fprintf(&sb, "[%s:%s:b]  v%s  [-:-:-]  [%s::b]%s[::-]\n\n",
				bg, focusColor, e.Version, fg, e.Title)
		}

		groups := groupChanges(e.Changes)
		for j, g := range groups {
			colors, ok := categoryColors[g.key]
			if ok {
				fmt.Fprintf(&sb, "[%s:%s:b]  %s  %s  [-:-:-]\n\n", colors[0], colors[1], g.symbol, g.label)
			} else {
				fmt.Fprintf(&sb, "[%s:%s:b]  %s  %s  [-:-:-]\n\n", fg, contrastBg, g.symbol, g.label)
			}
			for _, item := range g.items {
				fmt.Fprintf(&sb, "  [%s]▸[-] %s\n", dimColor, item)
			}
			if j < len(groups)-1 {
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}
