package config

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
		FocusUp: Key{
			Keys:        []string{"Ctrl+k", "Backtab"},
			Description: "Focus up component/form field",
		},
		FocusDown: Key{
			Keys:        []string{"Ctrl+j", "Tab"},
			Description: "Focus down component/form field",
		},
		FocusLeft: Key{
			Keys:        []string{"Ctrl+h", "Alt+Left"},
			Description: "Focus left component",
		},
		FocusRight: Key{
			Keys:        []string{"Ctrl+l", "Alt+Right"},
			Description: "Focus right component",
		},
		AutocompleteUp: Key{
			Keys:        []string{"Ctrl+p", "Up"},
			Description: "Autocomplete up",
		},
		AutocompleteDown: Key{
			Keys:        []string{"Ctrl+n", "Down"},
			Description: "Autocomplete down",
		},
		AutocompleteAccept: Key{
			Keys:        []string{"Ctrl+y", "Enter"},
			Description: "Autocomplete accept",
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
		AddConnection: Key{
			Runes:       []string{"a"},
			Description: "Add new connection",
		},
		DeleteConnection: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete selected connection",
		},
		EditConnection: Key{
			Runes:       []string{"e"},
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
		EditKey: Key{
			Runes:       []string{"e"},
			Description: "Edit keybinding",
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
			Description: "Only value",
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

	k.CreateTable = CreateTableKeys{
		AddColumn: Key{
			Runes:       []string{"a"},
			Description: "Add column",
		},
		DeleteColumn: Key{
			Runes:       []string{"d"},
			Description: "Delete column",
		},
		Execute: Key{
			Keys:        []string{"Ctrl+s"},
			Description: "Create table",
		},
		Cancel: Key{
			Keys:        []string{"Esc"},
			Description: "Cancel",
		},
	}
}
