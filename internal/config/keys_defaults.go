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
			Description: "Focus up",
		},
		FocusDown: Key{
			Keys:        []string{"Ctrl+j", "Tab"},
			Description: "Focus down",
		},
		FocusLeft: Key{
			Keys:        []string{"Ctrl+h", "Backtab"},
			Description: "Focus left",
		},
		FocusRight: Key{
			Keys:        []string{"Ctrl+l", "Tab"},
			Description: "Focus right",
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
			Description: "Close app",
		},
		FullScreenHelp: Key{
			Runes:       []string{"?"},
			Description: "Help page",
		},
		OpenConnection: Key{
			Keys:        []string{"Ctrl+o"},
			Description: "Open connections",
		},
		ChangeStyle: Key{
			Keys:        []string{"Alt+t"},
			Description: "Change style",
		},
		ToggleFooter: Key{
			Keys:        []string{"Ctrl+t"},
			Description: "Expand/collapse footer",
		},
		HideSchema: Key{
			Runes:       []string{"|"},
			Description: "Hide schemas",
		},
		ServerInfo: Key{
			Keys:        []string{"Alt+s"},
			Description: "Server info",
		},
		NewTab: Key{
			Keys:        []string{"Ctrl+a"},
			Description: "New tab",
		},
		CloseTab: Key{
			Keys:        []string{"Ctrl+x"},
			Description: "Close tab",
		},
		FocusSchemaTree: Key{
			Keys:        []string{"Ctrl+b"},
			Description: "Focus schema tree",
		},
		OpenActions: Key{
			Keys:        []string{"Ctrl+p"},
			Description: "Open actions",
		},
	}

	k.Schema = SchemaKeys{
		FilterBar: Key{
			Runes:       []string{"/"},
			Description: "Filter bar",
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
			Runes:       []string{"a"},
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
		ExpandTable: Key{
			Runes:       []string{"e"},
			Description: "Expand table",
		},
	}

	k.InputBar = InputBarKeys{
		Exit: Key{
			Keys:        []string{"Esc"},
			Description: "Close",
		},
		ClearInput: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+v"},
			Description: "Paste",
		},
	}

	k.Data = DataKeys{
		PeekRow: Key{
			Runes:       []string{"o"},
			Keys:        []string{"Enter"},
			Description: "Peek",
		},
		FullPagePeek: Key{
			Runes:       []string{"O"},
			Description: "Full peek",
		},
		TermEditor: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "$EDITOR",
		},
		AddRow: Key{
			Runes:       []string{"a"},
			Description: "Add new",
		},
		EditRow: Key{
			Runes:       []string{"E"},
			Description: "Edit",
		},
		InlineEdit: Key{
			Runes:       []string{"e"},
			Description: "Inline edit",
		},
		DuplicateRow: Key{
			Runes:       []string{"D"},
			Description: "Duplicate",
		},
		DeleteRow: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete",
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
			Description: "Copy",
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
		ToggleSortBar: Key{
			Runes:       []string{"s"},
			Description: "Sort bar",
		},
		SortByColumn: Key{
			Runes:       []string{"S"},
			Description: "Sort by col",
		},
		HideColumn: Key{
			Runes:       []string{"H"},
			Description: "Hide col",
		},
		ResetHiddenColumns: Key{
			Runes:       []string{"r"},
			Description: "Reset cols",
		},
		NextPage: Key{
			Runes:       []string{"n"},
			Description: "Next page",
		},
		PreviousPage: Key{
			Runes:       []string{"b"},
			Description: "Previous page",
		},
		ExplainQuery: Key{
			Keys:        []string{"Ctrl+g"},
			Description: "Explain query",
		},
		ExportData: Key{
			Keys:        []string{"Ctrl+m"},
			Description: "Export data",
		},
	}

	k.ExplainViewer = ExplainViewerKeys{
		Close: Key{
			Keys:        []string{"Esc"},
			Description: "Close",
		},
		ToggleMode: Key{
			Runes:       []string{"t"},
			Description: "Toggle ANALYZE mode",
		},
	}

	k.Connection.ConnectionForm = ConnectionFormKeys{
		SaveConnection: Key{
			Keys:        []string{"Ctrl+s"},
			Description: "Save",
		},
	}

	k.Connection.ConnectionList = ConnectionListKeys{
		AddConnection: Key{
			Runes:       []string{"a"},
			Description: "Add new",
		},
		DeleteConnection: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete",
		},
		EditConnection: Key{
			Runes:       []string{"e"},
			Description: "Edit",
		},
		SetConnection: Key{
			Keys:        []string{"Enter", "Space"},
			Description: "Set selected",
		},
	}

	k.Help = HelpKeys{
		Close: Key{
			Keys:        []string{"Esc"},
			Description: "Close",
		},
		Search: Key{
			Runes:       []string{"/"},
			Description: "Search",
		},
		EditKey: Key{
			Runes:       []string{"e"},
			Description: "Edit key",
		},
	}

	k.Peeker = PeekerKeys{
		MoveToTop: Key{
			Runes:       []string{"g"},
			Description: "Go to top",
		},
		MoveToBottom: Key{
			Runes:       []string{"G"},
			Description: "Go to bottom",
		},
		CopyValue: Key{
			Runes:       []string{"c"},
			Description: "Copy value",
		},
		CopyHighlight: Key{
			Runes:       []string{"C"},
			Description: "Copy highlight",
		},
		ExpandRow: Key{
			Keys:        []string{"Enter"},
			Description: "Expand",
		},
		OpenValueViewer: Key{
			Runes:       []string{"v"},
			Description: "Viewer",
		},
		ToggleFullScreen: Key{
			Runes:       []string{"f"},
			Description: "Full screen",
		},
		Exit: Key{
			Keys:        []string{"Esc"},
			Runes:       []string{"q"},
			Description: "Close",
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
			Description: "Close",
		},
		DeleteEntry: Key{
			Runes:       []string{"d"},
			Description: "Delete entry",
		},
		CopyQuery: Key{
			Runes:       []string{"c"},
			Description: "Copy query",
		},
	}

	k.Index = IndexKeys{
		AddIndex: Key{
			Runes:       []string{"a"},
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
			Description: "Close",
		},
		ToggleSQLMode: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "SQL mode",
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
			Description: "Refresh",
		},
		RenameColumn: Key{
			Runes:       []string{"R"},
			Description: "Rename column",
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
			Description: "Close",
		},
	}

	k.SQLQueryEditor = SQLQueryEditorKeys{
		Execute: Key{
			Keys:        []string{"Ctrl+Enter"},
			Description: "Execute",
		},
		Clear: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear",
		},
		Expand: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "Resize editor",
		},
		OpenHistory: Key{
			Keys:        []string{"Ctrl+r"},
			Description: "Query history",
		},
	}
}
