package config

func (k *KeyBindings) loadDefaults() {
	k.Common = CommonKeys{
		Close: Key{
			Keys:        []string{"Esc"},
			Description: "Close",
		},
		Delete: Key{
			Keys:        []string{"Ctrl+d"},
			Description: "Delete",
		},
		Add: Key{
			Runes:       []string{"a"},
			Description: "Add",
		},
		Edit: Key{
			Runes:       []string{"e"},
			Description: "Edit",
		},
		Filter: Key{
			Runes:       []string{"/"},
			Description: "Filter",
		},
		Select: Key{
			Keys:        []string{"Enter", "Space"},
			Description: "Select",
		},
		Copy: Key{
			Runes:       []string{"c"},
			Description: "Copy",
		},
		Confirm: Key{
			Keys:        []string{"Ctrl+s"},
			Description: "Confirm",
		},
		Refresh: Key{
			Keys:        []string{"Ctrl+r"},
			Description: "Refresh",
		},
		Clear: Key{
			Keys:        []string{"Ctrl+u"},
			Description: "Clear",
		},
		Paste: Key{
			Keys:        []string{"Ctrl+v"},
			Description: "Paste",
		},
	}

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
			Description: "Close app",
		},
		FullScreenHelp: Key{
			Keys:        []string{"F1"},
			Description: "Help",
		},
		ToggleFooter: Key{
			Keys:        []string{"Alt+f"},
			Description: "Expand/collapse footer",
		},
		OpenConnection: Key{
			Keys:        []string{"Ctrl+o"},
			Description: "Open connections",
		},
		ChangeStyle: Key{
			Keys:        []string{"Alt+t"},
			Description: "Change style",
		},
	}

	k.Main = MainKeys{
		ServerInfo: Key{
			Keys:        []string{"Alt+s"},
			Description: "Server info",
		},
		HideSchema: Key{
			Runes:       []string{"|"},
			Description: "Hide schemas",
		},
		NewTab: Key{
			Keys:        []string{"Ctrl+t"},
			Description: "New tab",
		},
		CloseTab: Key{
			Keys:        []string{"Ctrl+x"},
			Description: "Close tab",
		},
		RenameTab: Key{
			Keys:        []string{"F2"},
			Description: "Rename tab",
		},
		FocusSchemaTree: Key{
			Keys:        []string{"Ctrl+/"},
			Description: "Focus schema tree",
		},
		OpenActions: Key{
			Keys:        []string{"Ctrl+Space"},
			Description: "Actions",
		},
		ImportData: Key{
			Keys:        []string{"Alt+i"},
			Description: "Import CSV",
		},
	}

	k.Schema = SchemaKeys{
		ExpandAll: Key{
			Runes:       []string{"E"},
			Description: "Expand all",
		},
		CollapseAll: Key{
			Runes:       []string{"W"},
			Description: "Collapse all",
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

	k.Data = DataKeys{
		PeekRow: Key{
			Runes:       []string{"o"},
			Keys:        []string{"Enter"},
			Description: "Peek row",
		},
		FullPagePeek: Key{
			Runes:       []string{"O"},
			Description: "Full peek",
		},
		EditRow: Key{
			Runes:       []string{"E"},
			Description: "Edit",
		},
		DuplicateRow: Key{
			Runes:       []string{"D"},
			Description: "Duplicate",
		},
		MultipleSelect: Key{
			Runes:       []string{"V"},
			Description: "Multiple select",
		},
		ClearSelection: Key{
			Keys:        []string{"Esc"},
			Description: "Clear selection",
		},
		CopyRow: Key{
			Runes:       []string{"C"},
			Description: "Copy row",
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
			Keys:        []string{"Alt+m"},
			Description: "Export data",
		},
		FollowForeignKey: Key{
			Keys:        []string{"Ctrl+b"},
			Description: "Follow FK",
		},
	}

	k.ExplainViewer = ExplainViewerKeys{
		ToggleMode: Key{
			Runes:       []string{"t"},
			Description: "Toggle ANALYZE mode",
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
	}

	k.History = HistoryKeys{
		PurgeHistory: Key{
			Keys:        []string{"Alt+d"},
			Description: "Purge history",
		},
		CopyQuery: Key{
			Runes:       []string{"c"},
			Description: "Copy query",
		},
	}

	k.IndexAddForm = IndexAddFormKeys{
		ToggleSQLMode: Key{
			Keys:        []string{"Alt+e"},
			Description: "SQL mode",
		},
		AddColumn: Key{
			Keys:        []string{"Ctrl+a"},
			Description: "Add column",
		},
	}

	k.Structure = StructureKeys{
		RenameColumn: Key{
			Runes:       []string{"R"},
			Description: "Rename column",
		},
		ToggleDDLPane: Key{
			Runes:       []string{"p"},
			Description: "Toggle DDL",
		},
	}

	k.SQLQueryEditor = SQLQueryEditorKeys{
		Expand: Key{
			Keys:        []string{"Alt+z"},
			Description: "Resize editor",
		},
		OpenHistory: Key{
			Keys:        []string{"Ctrl+m"},
			Description: "Query history",
		},
		TermEditor: Key{
			Keys:        []string{"Ctrl+e"},
			Description: "Open in $EDITOR",
		},
	}
}
