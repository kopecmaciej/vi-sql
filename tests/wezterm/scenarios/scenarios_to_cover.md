# Scenarios not yet covered by e2e tests

- Master password: setup, unlock on startup, reset flow
- MCP HTTP endpoints: start server, execute_query, open_query_in_tab (needs HTTP client in test)
- CSV import edge cases: bad header, type mismatches, encoding issues
- EXPLAIN viewer: run EXPLAIN and verify tree/plan rendering
- Vim mode: verify distinct keybindings from normal mode
- Term editor: open external editor, write query, return to TUI
- Foreign key follow / find references
