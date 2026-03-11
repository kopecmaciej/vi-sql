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
	OrderedKeys struct {
		Element string
		Keys    []Key
	}

	KeyBindings struct {
		Global       GlobalKeys       `yaml:"global"`
		Help         HelpKeys         `yaml:"help"`
		Welcome      WelcomeKeys      `yaml:"welcome"`
		Connection   ConnectionKeys   `yaml:"connection"`
		Main         MainKeys         `yaml:"main"`
		Schema       SchemaKeys       `yaml:"schema"`
		FilterBar    FilterBarKeys    `yaml:"filterBar"`
		Content      ContentKeys      `yaml:"content"`
		Peeker       PeekerKeys       `yaml:"peeker"`
		QueryBar     QueryBar         `yaml:"queryBar"`
		SortBar      SortBar          `yaml:"sortBar"`
		Index        IndexKeys        `yaml:"index"`
		IndexAddForm IndexAddFormKeys `yaml:"indexAddForm"`
		Structure    StructureKeys    `yaml:"structure"`
		AIQuery      AIQuery          `yaml:"aiPrompt"`
		History      HistoryKeys      `yaml:"history"`
	}

	Key struct {
		Keys        []string `yaml:"keys,omitempty,flow"`
		Runes       []string `yaml:"runes,omitempty,flow"`
		Description string   `yaml:"description"`
	}

	GlobalKeys struct {
		CloseApp             Key `yaml:"closeApp"`
		ToggleFullScreenHelp Key `yaml:"toggleFullScreenHelp"`
		OpenConnection       Key `yaml:"openConnection"`
		ShowStyleModal       Key `yaml:"showStyleModal"`
		ToggleHeader         Key `yaml:"toggleHeader"`
	}

	MainKeys struct {
		FocusNext      Key `yaml:"focusNext"`
		FocusPrevious  Key `yaml:"focusPrevious"`
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

	FilterBarKeys struct {
		CloseFilter Key `yaml:"closeFilter"`
		ClearFilter Key `yaml:"clearFilter"`
	}

	ContentKeys struct {
		ChangeView            Key `yaml:"switchView"`
		PeekRow               Key `yaml:"peekRow"`
		FullPagePeek          Key `yaml:"fullPagePeek"`
		OpenEditor            Key `yaml:"openEditor"`
		AddRow                Key `yaml:"addRow"`
		EditRow               Key `yaml:"editRow"`
		InlineEdit            Key `yaml:"inlineEdit"`
		DuplicateRow          Key `yaml:"duplicateRow"`
		DuplicateRowNoConfirm Key `yaml:"duplicateRowNoConfirm"`
		DeleteRow             Key `yaml:"deleteRow"`
		DeleteRowNoConfirm    Key `yaml:"deleteRowNoConfirm"`
		CopyHighlight         Key `yaml:"copyValue"`
		CopyRow               Key `yaml:"copyRow"`
		Refresh               Key `yaml:"refresh"`
		ToggleFilterBar       Key `yaml:"toggleFilterBar"`
		ToggleQueryBar        Key `yaml:"toggleQueryBar"`
		NextRow               Key `yaml:"nextRow"`
		PreviousRow           Key `yaml:"previousRow"`
		NextPage              Key `yaml:"nextPage"`
		PreviousPage          Key `yaml:"previousPage"`
		ToggleSortBar         Key `yaml:"toggleSortBar"`
		SortByColumn          Key `yaml:"sortByColumn"`
		HideColumn            Key `yaml:"hideColumn"`
		ResetHiddenColumns    Key `yaml:"resetHiddenColumns"`
		ToggleFilterOptions   Key `yaml:"toggleFilterOptions"`
		MultipleSelect        Key `yaml:"multipleSelect"`
		ClearSelection        Key `yaml:"clearSelection"`
	}

	QueryBar struct {
		ShowHistory Key `yaml:"showHistory"`
		ClearInput  Key `yaml:"clearInput"`
		Paste       Key `yaml:"paste"`
	}

	SortBar struct {
		ClearInput Key `yaml:"clearInput"`
		Paste      Key `yaml:"paste"`
	}

	ConnectionKeys struct {
		ToggleFocus    Key                `yaml:"toggleFocus"`
		ConnectionForm ConnectionFormKeys `yaml:"connectionForm"`
		ConnectionList ConnectionListKeys `yaml:"connectionList"`
	}

	ConnectionFormKeys struct {
		SaveConnection Key `yaml:"saveConnection"`
		FocusList      Key `yaml:"focusList"`
	}

	ConnectionListKeys struct {
		FocusForm        Key `yaml:"focusForm"`
		DeleteConnection Key `yaml:"deleteConnection"`
		EditConnection   Key `yaml:"editConnection"`
		SetConnection    Key `yaml:"setConnection"`
	}

	WelcomeKeys struct {
		MoveFocusUp   Key `yaml:"moveFocusUp"`
		MoveFocusDown Key `yaml:"moveFocusDown"`
	}

	HelpKeys struct {
		Close Key `yaml:"close"`
	}

	PeekerKeys struct {
		CopyHighlight    Key `yaml:"copyHighlight"`
		CopyValue        Key `yaml:"copyValue"`
		ExpandRow        Key `yaml:"expandRow"`
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
		ExitAddIndex Key `yaml:"exitModal"`
		AddIndex     Key `yaml:"addIndex"`
		DeleteIndex  Key `yaml:"deleteIndex"`
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

	AIQuery struct {
		ExitAIQuery Key `yaml:"exitAIQuery"`
		ClearPrompt Key `yaml:"clearPrompt"`
	}
)

const keybindingsFileHeader = `# runes: literal characters, case-sensitive (e.g. [a], [A])
# keys:  named/combo keys (e.g. [Enter], [Escape], [Tab], [Space])
#        Ctrl+<letter>: case-insensitive in config, but no Ctrl+Shift — use lowercase (e.g. Ctrl+l)
#        Alt+<char>:    case-sensitive, both upper and lower work (e.g. Alt+a, Alt+A)

`

func (k *KeyBindings) loadDefaults() {
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
		FocusNext: Key{
			Keys:        []string{"Ctrl+l", "Tab"},
			Description: "Focus next component",
		},
		FocusPrevious: Key{
			Keys:        []string{"Ctrl+h", "Backtab"},
			Description: "Focus previous component",
		},
		HideSchema: Key{
			Keys:        []string{"Ctrl+n"},
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

	k.FilterBar = FilterBarKeys{
		CloseFilter: Key{
			Keys:        []string{"Escape"},
			Description: "Close filter bar",
		},
		ClearFilter: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear filter",
		},
	}

	k.Content = ContentKeys{
		ChangeView: Key{
			Runes:       []string{"v"},
			Description: "Change view",
		},
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
		DuplicateRowNoConfirm: Key{
			Keys:        []string{"Alt+D"},
			Description: "Duplicate without confirmation",
		},
		DeleteRow: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete row",
		},
		DeleteRowNoConfirm: Key{
			Keys:        []string{"Alt+d"},
			Description: "Delete without confirmation",
		},
		MultipleSelect: Key{
			Runes:       []string{"V"},
			Description: "Multiple select",
		},
		ClearSelection: Key{
			Keys:        []string{"Esc"},
			Description: "Clear selection",
		},
		CopyHighlight: Key{
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
		NextRow: Key{
			Runes:       []string{"]"},
			Description: "Next row",
		},
		PreviousRow: Key{
			Runes:       []string{"["},
			Description: "Previous row",
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
		ClearInput: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear input",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+v"},
			Description: "Paste from clipboard",
		},
	}

	k.SortBar = SortBar{
		ClearInput: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear input",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+v"},
			Description: "Paste from clipboard",
		},
	}

	k.Connection.ToggleFocus = Key{
		Keys:        []string{"Tab", "Backtab"},
		Description: "Toggle focus",
	}

	k.Connection.ConnectionForm = ConnectionFormKeys{
		SaveConnection: Key{
			Keys:        []string{"Ctrl+s"},
			Description: "Save connection",
		},
		FocusList: Key{
			Keys:        []string{"Ctrl+h", "Ctrl+Left"},
			Description: "Focus Connection List",
		},
	}

	k.Connection.ConnectionList = ConnectionListKeys{
		FocusForm: Key{
			Keys:        []string{"Ctrl+l", "Ctrl+Right"},
			Description: "Move focus to form",
		},
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

	k.Welcome = WelcomeKeys{
		MoveFocusUp: Key{
			Keys:        []string{"Backtab"},
			Description: "Move focus up",
		},
		MoveFocusDown: Key{
			Keys:        []string{"Tab"},
			Description: "Move focus down",
		},
	}

	k.Help = HelpKeys{
		Close: Key{
			Keys:        []string{"Esc"},
			Description: "Close help",
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
		CopyHighlight: Key{
			Runes:       []string{"c"},
			Description: "Copy highlighted",
		},
		CopyValue: Key{
			Runes:       []string{"C"},
			Description: "Copy only value",
		},
		ExpandRow: Key{
			Keys:        []string{"Enter"},
			Description: "Expand row value",
		},
		ToggleFullScreen: Key{
			Runes:       []string{"F"},
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
		ExitAddIndex: Key{
			Keys:        []string{"Esc"},
			Description: "Exit modal",
		},
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

	k.AIQuery = AIQuery{
		ExitAIQuery: Key{
			Keys:        []string{"Esc"},
			Description: "Exit AI query",
		},
		ClearPrompt: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear prompt",
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

	keys := []OrderedKeys{{
		Element: elementId,
		Keys:    extractKeysFromStruct(field),
	}}

	return keys, nil
}

func (kb *KeyBindings) ConvertStrKeyToTcellKey(key string) (tcell.Key, bool) {
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
