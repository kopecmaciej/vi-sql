package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"gopkg.in/yaml.v3"
)

type (
	Key struct {
		Keys        []string `yaml:"keys,omitempty,flow"`
		Runes       []string `yaml:"runes,omitempty,flow"`
		Description string   `yaml:"description"`
	}

	OrderedKeys struct {
		Element string
		Keys    []Key
	}

	KeyBindings struct {
		Navigation   NavigationKeys   `yaml:"navigation"`
		Global       GlobalKeys       `yaml:"global"`
		Help         HelpKeys         `yaml:"help"`
		Connection   ConnectionKeys   `yaml:"connection"`
		Main         MainKeys         `yaml:"main"`
		Schema       SchemaKeys       `yaml:"schema"`
		InputBar     InputBarKeys     `yaml:"inputBar"`
		Content      ContentKeys      `yaml:"content"`
		Peeker       PeekerKeys       `yaml:"peeker"`
		QueryBar     QueryBar         `yaml:"queryBar"`
		Index        IndexKeys        `yaml:"index"`
		IndexAddForm IndexAddFormKeys `yaml:"indexAddForm"`
		Structure    StructureKeys    `yaml:"structure"`
		History      HistoryKeys      `yaml:"history"`
	}

	NavigationKeys struct {
		MoveDown        Key `yaml:"moveDown"`
		MoveUp          Key `yaml:"moveUp"`
		MoveLeft        Key `yaml:"moveLeft"`
		MoveRight       Key `yaml:"moveRight"`
		FocusLeft       Key `yaml:"focusLeft"`
		FocusRight      Key `yaml:"focusRight"`
		DropdownUp      Key `yaml:"dropdownUp"`
		DropdownDown    Key `yaml:"dropdownDown"`
		DropdownAccept  Key `yaml:"dropdownAccept"`
		DropdownDismiss Key `yaml:"dropdownDismiss"`
	}

	GlobalKeys struct {
		CloseApp             Key `yaml:"closeApp"`
		ToggleFullScreenHelp Key `yaml:"toggleFullScreenHelp"`
		OpenConnection       Key `yaml:"openConnection"`
		ShowStyleModal       Key `yaml:"showStyleModal"`
		ToggleHeader         Key `yaml:"toggleHeader"`
	}

	MainKeys struct {
		HideSchema     Key `yaml:"hideSchema"`
		ShowAIQuery    Key `yaml:"showAIQuery"`
		ShowServerInfo Key `yaml:"showServerInfo"`
	}

	SchemaKeys struct {
		FilterBar   Key `yaml:"filterBar"`
		ClearFilter Key `yaml:"clearFilter"`
		ExpandAll   Key `yaml:"expandAll"`
		CollapseAll Key `yaml:"collapseAll"`
		AddTable    Key `yaml:"addTable"`
		DeleteTable Key `yaml:"deleteTable"`
		RenameTable Key `yaml:"renameTable"`
	}

	InputBarKeys struct {
		Exit       Key `yaml:"exit"`
		ClearInput Key `yaml:"clearInput"`
		Paste      Key `yaml:"paste"`
	}

	ContentKeys struct {
		PeekRow             Key `yaml:"peekRow"`
		FullPagePeek        Key `yaml:"fullPagePeek"`
		OpenEditor          Key `yaml:"openEditor"`
		AddRow              Key `yaml:"addRow"`
		EditRow             Key `yaml:"editRow"`
		InlineEdit          Key `yaml:"inlineEdit"`
		DuplicateRow        Key `yaml:"duplicateRow"`
		DeleteRow           Key `yaml:"deleteRow"`
		CopyValue           Key `yaml:"copyValue"`
		CopyRow             Key `yaml:"copyRow"`
		Refresh             Key `yaml:"refresh"`
		ToggleFilterBar     Key `yaml:"toggleFilterBar"`
		ToggleQueryBar      Key `yaml:"toggleQueryBar"`
		NextPage            Key `yaml:"nextPage"`
		PreviousPage        Key `yaml:"previousPage"`
		ToggleSortBar       Key `yaml:"toggleSortBar"`
		SortByColumn        Key `yaml:"sortByColumn"`
		HideColumn          Key `yaml:"hideColumn"`
		ResetHiddenColumns  Key `yaml:"resetHiddenColumns"`
		ToggleFilterOptions Key `yaml:"toggleFilterOptions"`
		MultipleSelect      Key `yaml:"multipleSelect"`
		ClearSelection      Key `yaml:"clearSelection"`
	}

	QueryBar struct {
		ShowHistory Key `yaml:"showHistory"`
	}

	ConnectionKeys struct {
		ConnectionForm ConnectionFormKeys `yaml:"connectionForm"`
		ConnectionList ConnectionListKeys `yaml:"connectionList"`
	}

	ConnectionFormKeys struct {
		SaveConnection Key `yaml:"saveConnection"`
	}

	ConnectionListKeys struct {
		DeleteConnection Key `yaml:"deleteConnection"`
		EditConnection   Key `yaml:"editConnection"`
		SetConnection    Key `yaml:"setConnection"`
	}

	HelpKeys struct {
		Close  Key `yaml:"close"`
		Search Key `yaml:"search"`
	}

	PeekerKeys struct {
		CopyHighlight    Key `yaml:"copyHighlight"`
		CopyValue        Key `yaml:"copyValue"`
		ExpandRow        Key `yaml:"expandRow"`
		OpenValueViewer  Key `yaml:"openValueViewer"`
		ToggleFullScreen Key `yaml:"toggleFullScreen"`
		Exit             Key `yaml:"exit"`
		MoveToTop        Key `yaml:"moveToTop"`
		MoveToBottom     Key `yaml:"moveToBottom"`
	}

	HistoryKeys struct {
		ClearHistory Key `yaml:"clearHistory"`
		AcceptEntry  Key `yaml:"acceptEntry"`
		CloseHistory Key `yaml:"closeHistory"`
	}

	IndexKeys struct {
		AddIndex    Key `yaml:"addIndex"`
		DeleteIndex Key `yaml:"deleteIndex"`
	}

	IndexAddFormKeys struct {
		ExitForm      Key `yaml:"exitForm"`
		ToggleSQLMode Key `yaml:"toggleSQLMode"`
		AddColumn     Key `yaml:"addColumn"`
		CreateIndex   Key `yaml:"createIndex"`
	}

	StructureKeys struct {
		Refresh Key `yaml:"refresh"`
	}

	AIQuery struct{}
)

// keyGroupParents defines optional single-parent inheritance for key groups.
// GetKeysForElement prepends the parent's keys before the child's own keys,
// making the header and help page show the full effective key set.
var keyGroupParents = map[string]string{
	// "ChildKeys": "ParentKeys"
	"QueryBar": "InputBar",
}

const keybindingsFileHeader = `# runes: literal characters, case-sensitive (e.g. [a], [A])
# keys:  named/combo keys (e.g. [Enter], [Esc], [Tab], [Space])
#        Ctrl+<letter>: case-insensitive in config, but no Ctrl+Shift — use lowercase (e.g. Ctrl+l)
#        Alt+<char>:    case-sensitive, both upper and lower work (e.g. Alt+a, Alt+A)

`

func (k *KeyBindings) loadDefaults() {
	k.Navigation = NavigationKeys{
		MoveUp: Key{
			Runes:       []string{"k"},
			Keys:        []string{"Up"},
			Description: "Move up",
		},
		MoveDown: Key{
			Runes:       []string{"j"},
			Keys:        []string{"Down"},
			Description: "Move down",
		},
		MoveLeft: Key{
			Runes:       []string{"h"},
			Keys:        []string{"Left"},
			Description: "Move left",
		},
		MoveRight: Key{
			Runes:       []string{"l"},
			Keys:        []string{"Right"},
			Description: "Move right",
		},
		FocusLeft: Key{
			Keys:        []string{"Ctrl+h", "Alt+Left"},
			Description: "Focus left component",
		},
		FocusRight: Key{
			Keys:        []string{"Ctrl+l", "Alt+Right"},
			Description: "Focus right component",
		},
		DropdownUp: Key{
			Keys:        []string{"Ctrl+p", "Up"},
			Description: "Dropdown up",
		},
		DropdownDown: Key{
			Keys:        []string{"Ctrl+n", "Down"},
			Description: "Dropdown down",
		},
		DropdownAccept: Key{
			Keys:        []string{"Ctrl+y", "Enter"},
			Description: "Dropdown accept",
		},
		DropdownDismiss: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "Dropdown dismiss",
		},
	}
	k.Global = GlobalKeys{
		CloseApp: Key{
			Keys:        []string{"Ctrl+c"},
			Runes:       []string{"q"},
			Description: "Close application",
		},
		ToggleFullScreenHelp: Key{
			Runes:       []string{"?"},
			Description: "Toggle full screen help",
		},
		OpenConnection: Key{
			Keys:        []string{"Ctrl+o"},
			Description: "Open connection page",
		},
		ShowStyleModal: Key{
			Keys:        []string{"Ctrl+t"},
			Description: "Toggle style change modal",
		},
		ToggleHeader: Key{
			Runes:       []string{"t"},
			Description: "Expand/collapse header",
		},
	}

	k.Main = MainKeys{
		HideSchema: Key{
			Runes:       []string{"|"},
			Description: "Hide schema panel",
		},
		ShowServerInfo: Key{
			Keys:        []string{"Alt+s"},
			Description: "Show server info",
		},
		ShowAIQuery: Key{
			Keys:        []string{"Alt+a"},
			Description: "Show AI prompt",
		},
	}

	k.Schema = SchemaKeys{
		FilterBar: Key{
			Runes:       []string{"/"},
			Description: "Focus filter bar",
		},
		ClearFilter: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear filter",
		},
		ExpandAll: Key{
			Runes:       []string{"E"},
			Description: "Expand all",
		},
		CollapseAll: Key{
			Runes:       []string{"W"},
			Description: "Collapse all",
		},
		AddTable: Key{
			Runes:       []string{"A"},
			Description: "Add table",
		},
		DeleteTable: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete table",
		},
		RenameTable: Key{
			Runes:       []string{"R"},
			Description: "Rename table",
		},
	}

	k.InputBar = InputBarKeys{
		Exit: Key{
			Keys:        []string{"Esc"},
			Description: "Close / cancel",
		},
		ClearInput: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear input",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+v"},
			Description: "Paste from clipboard",
		},
	}

	k.Content = ContentKeys{
		PeekRow: Key{
			Runes:       []string{"o"},
			Keys:        []string{"Enter"},
			Description: "Peek",
		},
		FullPagePeek: Key{
			Runes:       []string{"O"},
			Description: "Full peek",
		},
		OpenEditor: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "Open SQL editor",
		},
		AddRow: Key{
			Runes:       []string{"A"},
			Description: "Add new row",
		},
		EditRow: Key{
			Runes:       []string{"E"},
			Description: "Edit row in editor",
		},
		InlineEdit: Key{
			Runes:       []string{"e"},
			Description: "Inline edit cell",
		},
		DuplicateRow: Key{
			Runes:       []string{"D"},
			Description: "Duplicate row",
		},
		DeleteRow: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete row",
		},
		MultipleSelect: Key{
			Runes:       []string{"V"},
			Description: "Multiple select",
		},
		ClearSelection: Key{
			Keys:        []string{"Esc"},
			Description: "Clear selection",
		},
		CopyValue: Key{
			Runes:       []string{"c"},
			Description: "Copy highlighted",
		},
		CopyRow: Key{
			Runes:       []string{"C"},
			Description: "Copy row",
		},
		Refresh: Key{
			Keys:        []string{"Ctrl+r"},
			Description: "Refresh",
		},
		ToggleFilterBar: Key{
			Runes:       []string{"/"},
			Description: "Filter bar",
		},
		ToggleQueryBar: Key{
			Runes:       []string{":"},
			Description: "Toggle SQL query bar",
		},
		ToggleSortBar: Key{
			Runes:       []string{"s"},
			Description: "Sort bar",
		},
		SortByColumn: Key{
			Runes:       []string{"S"},
			Description: "Sort by current column",
		},
		HideColumn: Key{
			Runes:       []string{"H"},
			Description: "Hide current column",
		},
		ResetHiddenColumns: Key{
			Runes:       []string{"r"},
			Description: "Reset hidden columns",
		},
		NextPage: Key{
			Runes:       []string{"n"},
			Description: "Next page",
		},
		PreviousPage: Key{
			Runes:       []string{"b"},
			Description: "Previous page",
		},
		ToggleFilterOptions: Key{
			Keys:        []string{"Alt+o"},
			Description: "Toggle filter options",
		},
	}

	k.QueryBar = QueryBar{
		ShowHistory: Key{
			Keys:        []string{"Ctrl+y"},
			Description: "Show history",
		},
	}

	k.Connection.ConnectionForm = ConnectionFormKeys{
		SaveConnection: Key{
			Keys:        []string{"Ctrl+s"},
			Description: "Save connection",
		},
	}

	k.Connection.ConnectionList = ConnectionListKeys{
		DeleteConnection: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete selected connection",
		},
		EditConnection: Key{
			Runes:       []string{"E"},
			Description: "Edit selected connection",
		},
		SetConnection: Key{
			Keys:        []string{"Enter", "Space"},
			Description: "Set selected connection",
		},
	}

	k.Help = HelpKeys{
		Close: Key{
			Keys:        []string{"Esc"},
			Description: "Close help",
		},
		Search: Key{
			Runes:       []string{"/"},
			Description: "Search",
		},
	}

	k.Peeker = PeekerKeys{
		MoveToTop: Key{
			Runes:       []string{"g"},
			Description: "Move to top",
		},
		MoveToBottom: Key{
			Runes:       []string{"G"},
			Description: "Move to bottom",
		},
		CopyValue: Key{
			Runes:       []string{"c"},
			Description: "Copy only value",
		},
		CopyHighlight: Key{
			Runes:       []string{"C"},
			Description: "Copy highlighted",
		},
		ExpandRow: Key{
			Keys:        []string{"Enter"},
			Description: "Expand row value",
		},
		OpenValueViewer: Key{
			Runes:       []string{"v"},
			Description: "Open value in viewer",
		},
		ToggleFullScreen: Key{
			Runes:       []string{"f"},
			Description: "Toggle full screen",
		},
		Exit: Key{
			Runes:       []string{"o", "O"},
			Description: "Exit",
		},
	}

	k.History = HistoryKeys{
		ClearHistory: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Clear history",
		},
		AcceptEntry: Key{
			Keys:        []string{"Enter", "Space"},
			Description: "Accept entry",
		},
		CloseHistory: Key{
			Keys:        []string{"Esc", "Ctrl+y"},
			Description: "Close history",
		},
	}

	k.Index = IndexKeys{
		AddIndex: Key{
			Runes:       []string{"A"},
			Description: "Add index",
		},
		DeleteIndex: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete index",
		},
	}

	k.IndexAddForm = IndexAddFormKeys{
		ExitForm: Key{
			Keys:        []string{"Esc"},
			Description: "Exit form",
		},
		ToggleSQLMode: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "Edit SQL mode",
		},
		AddColumn: Key{
			Keys:        []string{"Ctrl+a"},
			Description: "Add column",
		},
		CreateIndex: Key{
			Keys:        []string{"Ctrl+s"},
			Description: "Create index",
		},
	}

	k.Structure = StructureKeys{
		Refresh: Key{
			Keys:        []string{"Ctrl+r"},
			Description: "Refresh structure",
		},
	}
}

func LoadKeybindings() (*KeyBindings, error) {
	defaultKeybindings := &KeyBindings{}
	defaultKeybindings.loadDefaults()

	if os.Getenv("ENV") == "vi-dev" {
		return defaultKeybindings, nil
	}

	keybindingsPath, err := getKeybindingsPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(keybindingsPath); os.IsNotExist(err) {
		if err := writeKeybindingsWithHeader(defaultKeybindings, keybindingsPath); err != nil {
			return nil, err
		}
		return defaultKeybindings, nil
	}

	return util.LoadConfigFile(defaultKeybindings, keybindingsPath)
}

func writeKeybindingsWithHeader(kb *KeyBindings, path string) error {
	data, err := yaml.Marshal(kb)
	if err != nil {
		return fmt.Errorf("failed to marshal keybindings: %w", err)
	}
	content := append([]byte(keybindingsFileHeader), data...)
	return os.WriteFile(path, content, FileMode)
}

func extractKeysFromStruct(val reflect.Value) []Key {
	var keys []Key

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Type() == reflect.TypeOf(Key{}) {
			keys = append(keys, field.Interface().(Key))
		} else if field.Kind() == reflect.Struct {
			keys = append(keys, extractKeysFromStruct(field)...)
		}
	}

	return keys
}

func (kb KeyBindings) GetAvailableKeys() []OrderedKeys {
	var keys []OrderedKeys

	v := reflect.ValueOf(kb)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldName := t.Field(i).Name

		orderedKeys := OrderedKeys{
			Element: fieldName,
			Keys:    extractKeysFromStruct(field),
		}

		keys = append(keys, orderedKeys)
	}

	return keys
}

func (kb KeyBindings) GetKeysForElement(elementId string) ([]OrderedKeys, error) {
	if elementId == "" {
		return nil, fmt.Errorf("element is empty")
	}

	v := reflect.ValueOf(kb)
	field := v.FieldByName(elementId)

	if !field.IsValid() || field.Kind() != reflect.Struct {
		return nil, fmt.Errorf("field %s not found", elementId)
	}

	var result []OrderedKeys
	if parent, ok := keyGroupParents[elementId]; ok {
		if parentKeys, err := kb.GetKeysForElement(parent); err == nil {
			result = append(result, parentKeys...)
		}
	}

	result = append(result, OrderedKeys{
		Element: elementId,
		Keys:    extractKeysFromStruct(field),
	})

	return result, nil
}

func (kb *KeyBindings) ConvertStrKeyToTcellKey(key string) (tcell.Key, bool) {
	// Normalize our config format to tcell's KeyNames format.
	// Config uses "Ctrl+N" (plus), tcell.KeyNames uses "Ctrl-N" (hyphen).
	if strings.HasPrefix(key, "Ctrl+") && len(key) == 6 {
		key = "Ctrl-" + strings.ToUpper(string(key[5]))
	}
	for k, v := range tcell.KeyNames {
		if v == key {
			return k, true
		}
	}
	return -1, false
}

func (kb *KeyBindings) Contains(configKey Key, namedKey string) bool {
	if namedKey == "Rune[ ]" {
		namedKey = "Space"
	}
	if namedKey == "Backspace" {
		namedKey = "Ctrl+H"
	}

	// Normalize Ctrl+letter to uppercase since tcell always reports uppercase,
	// allowing config to use lowercase (e.g. "Ctrl+l") for user clarity
	if strings.HasPrefix(namedKey, "Ctrl+") && len(namedKey) == 6 {
		namedKey = "Ctrl+" + strings.ToUpper(string(namedKey[5]))
	}
	if strings.HasPrefix(namedKey, "Alt+Rune[") && len(namedKey) >= 10 {
		runeChar := namedKey[9:10]
		altCombo := "Alt+" + runeChar

		for _, k := range configKey.Keys {
			if k == altCombo {
				return true
			}
		}
		return false
	}

	if strings.HasPrefix(namedKey, "Rune") {
		namedKey = strings.TrimPrefix(namedKey, "Rune")
		for _, k := range configKey.Runes {
			if k == namedKey[1:2] {
				return true
			}
		}
	}

	for _, k := range configKey.Keys {
		// Normalize Ctrl+letter to uppercase to match tcell's key naming
		if strings.HasPrefix(k, "Ctrl+") && len(k) == 6 {
			k = "Ctrl+" + strings.ToUpper(string(k[5]))
		}
		if k == namedKey {
			return true
		}
	}

	return false
}

func (k *Key) String() string {
	var keyString string
	var iter []string
	if len(k.Keys) > 0 {
		iter = k.Keys
	} else {
		iter = k.Runes
	}
	for i, k := range iter {
		if i == 0 {
			keyString = k
		} else {
			keyString = fmt.Sprintf("%s, %s", keyString, k)
		}
	}

	return keyString
}

func getKeybindingsPath() (string, error) {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}

	return configDir + "/keybindings.yaml", nil
}
