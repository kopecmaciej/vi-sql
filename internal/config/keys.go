package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

type (
	OrderedKeys struct {
		Element string
		Keys    []Key
	}

	KeyBindings struct {
		Global     GlobalKeys     `json:"global"`
		Help       HelpKeys       `json:"help"`
		Welcome    WelcomeKeys    `json:"welcome"`
		Connection ConnectionKeys `json:"connection"`
		Main       MainKeys       `json:"main"`
		Schema     SchemaKeys     `json:"schema"`
		FilterBar  FilterBarKeys  `json:"filterBar"`
		Content    ContentKeys    `json:"content"`
		Peeker     PeekerKeys     `json:"peeker"`
		QueryBar   QueryBar       `json:"queryBar"`
		SortBar    SortBar        `json:"sortBar"`
		Index      IndexKeys      `json:"index"`
		Structure  StructureKeys  `json:"structure"`
		AIQuery    AIQuery        `json:"aiPrompt"`
		History    HistoryKeys    `json:"history"`
	}

	Key struct {
		Keys        []string `json:"keys,omitempty"`
		Runes       []string `json:"runes,omitempty"`
		Description string   `json:"description"`
	}

	GlobalKeys struct {
		CloseApp             Key `json:"closeApp"`
		ToggleFullScreenHelp Key `json:"toggleFullScreenHelp"`
		OpenConnection       Key `json:"openConnection"`
		ShowStyleModal       Key `json:"showStyleModal"`
		ToggleHeader         Key `json:"toggleHeader"`
	}

	MainKeys struct {
		FocusNext      Key `json:"focusNext"`
		FocusPrevious  Key `json:"focusPrevious"`
		HideSchema     Key `json:"hideSchema"`
		ShowAIQuery    Key `json:"showAIQuery"`
		ShowServerInfo Key `json:"showServerInfo"`
	}

	SchemaKeys struct {
		FilterBar   Key `json:"filterBar"`
		ClearFilter Key `json:"clearFilter"`
		ExpandAll   Key `json:"expandAll"`
		CollapseAll Key `json:"collapseAll"`
		AddTable    Key `json:"addTable"`
		DeleteTable Key `json:"deleteTable"`
		RenameTable Key `json:"renameTable"`
	}

	FilterBarKeys struct {
		CloseFilter Key `json:"closeFilter"`
		ClearFilter Key `json:"clearFilter"`
	}

	ContentKeys struct {
		ChangeView            Key `json:"switchView"`
		PeekRow               Key `json:"peekRow"`
		FullPagePeek          Key `json:"fullPagePeek"`
		AddRow                Key `json:"addRow"`
		EditRow               Key `json:"editRow"`
		InlineEdit            Key `json:"inlineEdit"`
		DuplicateRow          Key `json:"duplicateRow"`
		DuplicateRowNoConfirm Key `json:"duplicateRowNoConfirm"`
		DeleteRow             Key `json:"deleteRow"`
		DeleteRowNoConfirm    Key `json:"deleteRowNoConfirm"`
		CopyHighlight         Key `json:"copyValue"`
		CopyRow               Key `json:"copyRow"`
		Refresh               Key `json:"refresh"`
		ToggleFilterBar       Key `json:"toggleFilterBar"`
		ToggleQueryBar        Key `json:"toggleQueryBar"`
		NextRow               Key `json:"nextRow"`
		PreviousRow           Key `json:"previousRow"`
		NextPage              Key `json:"nextPage"`
		PreviousPage          Key `json:"previousPage"`
		ToggleSortBar         Key `json:"toggleSortBar"`
		SortByColumn          Key `json:"sortByColumn"`
		HideColumn            Key `json:"hideColumn"`
		ResetHiddenColumns    Key `json:"resetHiddenColumns"`
		ToggleFilterOptions   Key `json:"toggleFilterOptions"`
		MultipleSelect        Key `json:"multipleSelect"`
		ClearSelection        Key `json:"clearSelection"`
	}

	QueryBar struct {
		ShowHistory Key `json:"showHistory"`
		ClearInput  Key `json:"clearInput"`
		Paste       Key `json:"paste"`
	}

	SortBar struct {
		ClearInput Key `json:"clearInput"`
		Paste      Key `json:"paste"`
	}

	ConnectionKeys struct {
		ToggleFocus    Key                `json:"toggleFocus"`
		ConnectionForm ConnectionFormKeys `json:"connectionForm"`
		ConnectionList ConnectionListKeys `json:"connectionList"`
	}

	ConnectionFormKeys struct {
		SaveConnection Key `json:"saveConnection"`
		FocusList      Key `json:"focusList"`
	}

	ConnectionListKeys struct {
		FocusForm        Key `json:"focusForm"`
		DeleteConnection Key `json:"deleteConnection"`
		EditConnection   Key `json:"editConnection"`
		SetConnection    Key `json:"setConnection"`
	}

	WelcomeKeys struct {
		MoveFocusUp   Key `json:"moveFocusUp"`
		MoveFocusDown Key `json:"moveFocusDown"`
	}

	HelpKeys struct {
		Close Key `json:"close"`
	}

	PeekerKeys struct {
		CopyHighlight    Key `json:"copyHighlight"`
		CopyValue        Key `json:"copyValue"`
		ExpandRow        Key `json:"expandRow"`
		ToggleFullScreen Key `json:"toggleFullScreen"`
		Exit             Key `json:"exit"`
		MoveToTop        Key `json:"moveToTop"`
		MoveToBottom     Key `json:"moveToBottom"`
	}

	HistoryKeys struct {
		ClearHistory Key `json:"clearHistory"`
		AcceptEntry  Key `json:"acceptEntry"`
		CloseHistory Key `json:"closeHistory"`
	}

	IndexKeys struct {
		ExitAddIndex Key `json:"exitModal"`
		AddIndex     Key `json:"addIndex"`
		DeleteIndex  Key `json:"deleteIndex"`
	}

	StructureKeys struct {
		Refresh Key `json:"refresh"`
	}

	AIQuery struct {
		ExitAIQuery Key `json:"exitAIQuery"`
		ClearPrompt Key `json:"clearPrompt"`
	}
)

func (k *KeyBindings) loadDefaults() {
	k.Global = GlobalKeys{
		CloseApp: Key{
			Keys:        []string{"Ctrl+C"},
			Runes:       []string{"q"},
			Description: "Close application",
		},
		ToggleFullScreenHelp: Key{
			Runes:       []string{"?"},
			Description: "Toggle full screen help",
		},
		OpenConnection: Key{
			Keys:        []string{"Ctrl+O"},
			Description: "Open connection page",
		},
		ShowStyleModal: Key{
			Keys:        []string{"Ctrl+T"},
			Description: "Toggle style change modal",
		},
		ToggleHeader: Key{
			Keys:        []string{"Ctrl+K"},
			Description: "Expand/collapse header keybindings",
		},
	}

	k.Main = MainKeys{
		FocusNext: Key{
			Keys:        []string{"Ctrl+L", "Tab"},
			Description: "Focus next component",
		},
		FocusPrevious: Key{
			Keys:        []string{"Ctrl+H", "Backtab"},
			Description: "Focus previous component",
		},
		HideSchema: Key{
			Keys:        []string{"Ctrl+N"},
			Description: "Hide schema panel",
		},
		ShowServerInfo: Key{
			Keys:        []string{"Ctrl+S"},
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
			Keys:        []string{"Ctrl+U"},
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
			Runes:       []string{"D"},
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
			Keys:        []string{"Ctrl+U"},
			Description: "Clear filter",
		},
	}

	k.Content = ContentKeys{
		ChangeView: Key{
			Runes:       []string{"f"},
			Description: "Change view",
		},
		PeekRow: Key{
			Runes:       []string{"p"},
			Keys:        []string{"Enter"},
			Description: "Quick peek",
		},
		FullPagePeek: Key{
			Runes:       []string{"P"},
			Description: "Full page peek",
		},
		AddRow: Key{
			Runes:       []string{"A"},
			Description: "Add new row",
		},
		EditRow: Key{
			Runes:       []string{"E"},
			Description: "Edit row",
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
			Runes:       []string{"d"},
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
			Runes:       []string{"y"},
			Description: "Copy highlighted",
		},
		CopyRow: Key{
			Runes:       []string{"Y"},
			Description: "Copy row",
		},
		Refresh: Key{
			Runes:       []string{"R"},
			Description: "Refresh",
		},
		ToggleFilterBar: Key{
			Runes:       []string{"/"},
			Description: "Toggle filter bar",
		},
		ToggleQueryBar: Key{
			Runes:       []string{":"},
			Description: "Toggle SQL query bar",
		},
		ToggleSortBar: Key{
			Runes:       []string{"s"},
			Description: "Toggle sort bar",
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
			Keys:        []string{"Ctrl+R"},
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
			Keys:        []string{"Ctrl+Y"},
			Description: "Show history",
		},
		ClearInput: Key{
			Keys:        []string{"Ctrl+D"},
			Description: "Clear input",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+V"},
			Description: "Paste from clipboard",
		},
	}

	k.SortBar = SortBar{
		ClearInput: Key{
			Keys:        []string{"Ctrl+D"},
			Description: "Clear input",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+V"},
			Description: "Paste from clipboard",
		},
	}

	k.Connection.ToggleFocus = Key{
		Keys:        []string{"Tab", "Backtab"},
		Description: "Toggle focus",
	}

	k.Connection.ConnectionForm = ConnectionFormKeys{
		SaveConnection: Key{
			Keys:        []string{"Ctrl+S"},
			Description: "Save connection",
		},
		FocusList: Key{
			Keys:        []string{"Ctrl+H", "Ctrl+Left"},
			Description: "Focus Connection List",
		},
	}

	k.Connection.ConnectionList = ConnectionListKeys{
		FocusForm: Key{
			Keys:        []string{"Ctrl+L", "Ctrl+Right"},
			Description: "Move focus to form",
		},
		DeleteConnection: Key{
			Runes:       []string{"D"},
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
			Runes:       []string{"y"},
			Description: "Copy highlighted",
		},
		CopyValue: Key{
			Runes:       []string{"Y"},
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
			Runes:       []string{"p", "P"},
			Description: "Exit",
		},
	}

	k.History = HistoryKeys{
		ClearHistory: Key{
			Runes:       []string{"D"},
			Description: "Clear history",
		},
		AcceptEntry: Key{
			Keys:        []string{"Enter", "Space"},
			Description: "Accept entry",
		},
		CloseHistory: Key{
			Keys:        []string{"Esc", "Ctrl+Y"},
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
			Runes:       []string{"D"},
			Description: "Delete index",
		},
	}

	k.Structure = StructureKeys{
		Refresh: Key{
			Runes:       []string{"R"},
			Description: "Refresh structure",
		},
	}

	k.AIQuery = AIQuery{
		ExitAIQuery: Key{
			Keys:        []string{"Esc"},
			Description: "Exit AI query",
		},
		ClearPrompt: Key{
			Keys:        []string{"Ctrl+D"},
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

	return util.LoadConfigFile(defaultKeybindings, keybindingsPath)
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

	return configDir + "/keybindings.json", nil
}
