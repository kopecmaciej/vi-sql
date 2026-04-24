# Wezterm-driven scenario tests — plan

A small harness that drives vi-sql by injecting keystrokes into a wezterm pane
and asserting against the log + rendered pane contents. Deliberately scoped to
one tool (wezterm) so future contributors can add other e2e approaches (PTY /
tmux / go-expect) under their own directory without fighting this one.

## Status

Working, local-only. Not runnable in CI. Tied to a maintainer-specific DB
fixture (`proxmox`). See "What needs to happen before this is public-ready"
below before advertising it to contributors.

## Current shape

- `harness/` — `Session` primitive (`Spawn`, `Send`, `Type`, `WaitForPane`,
  `WaitForLog`, `AssertPaneContains`, `AssertLogContains`, `Close`). Key
  encoder translates `"Ctrl+t"`, `"Enter"`, `"Down"` etc. into byte sequences.
- `scenarios/` — three scenarios (filter, jump, tab lifecycle). Guarded by
  `//go:build wezterm` so they never run under `go test ./...`.
- Env: `VI_SQL_WEZTERM_CONNECTION`, `VI_SQL_WEZTERM_JUMP`, `VI_SQL_WEZTERM_SLOW`,
  `VI_SQL_WEZTERM_KEY_DELAY_MS`, `VI_SQL_WEZTERM_BINARY`, `VI_SQL_WEZTERM_LOG_PATH`.
  Defaults for connection/jump live in `scenarios/main_test.go`.

## Known weaknesses (don't paper over)

1. **Screen-scraping is cosmetic-sensitive.** Asserting `" rows"` or
   `"WHERE: 1=1"` breaks when the results bar text changes. Most scenarios
   test *rendered strings*, not *behavior*.
2. **Low ROI per scenario.** `TestQueryTabLifecycle` presses `Ctrl+t`/`Ctrl+x`
   and checks a word. A unit test on the tab registry catches the same
   regression in 50ms with no wezterm.
3. **DB fixture is the maintainer's.** `proxmox` connection + `auth/users`
   table aren't portable. Anyone else cloning the repo gets skipped tests and
   no path to running them.
4. **No CI.** Without CI, scenarios rot. Rename a keybinding six months from
   now → tests break → nobody notices.
5. **PTY bypass.** `wezterm cli send-text` injects raw bytes into the terminal
   stream. Keybindings with ambiguous encodings (Ctrl+/, Shift+modified keys,
   CSI-u sequences) are inherently fragile. We already hit this with Ctrl+/.

## Roadmap — highest leverage first

### 1. MCP-based assertions

Biggest single win. vi-sql already runs an MCP server (`internal/mcp/server.go`)
that subscribes to `QueryExecuted` events. Replace pane-scraping with:

```go
last := s.MCP.WaitForQuery(10*time.Second)
assert.Equal(t, "catalog", last.Schema)
assert.Contains(t, last.Query, "WHERE")
```

Requires:
- **CLI flag to enable MCP for tests.** Add `--mcp-enabled` / `--mcp-port` in
  `cmd/cmd.go` (current behavior: config-driven only). Tests pick a free port
  and read it back from the log or stdout.
- **Sequence number on `QueryResult`.** Today MCP caches only the *last* query.
  `WaitForQuery` needs to know "which is new since my last wait". One-line
  addition in `internal/manager/messages.go` (`SeqNum int64` bumped per emit).
- **`harness/mcp.go`.** Small HTTP client for the MCP protocol. Exposes
  `WaitForQuery`, `GetLastQueryResult`, `OpenQueryInTab`, `ExecuteStatement`,
  `ListSchemas`.

### 2. Higher-level action helpers

Scenarios today read as "Down, Down, Enter, Down, Down, Enter" — that's the
wrong abstraction level. One keybinding change breaks every scenario.

```go
s.OpenTable("catalog", "price_rules")   // encapsulates focus + nav + expand + select
s.RunQuery("SELECT 1")                   // types + submits
s.TypeQuery(sql)                         // types into the active SQL editor
s.FocusSchemaTree()
s.Filter("id > 5")                       // press /, type, enter
```

A keybinding change then touches one helper, not every scenario. Implement in
`harness/actions.go`.

### 3. Portable DB fixture

Ship `tests/wezterm/fixture.sql` (SQLite) that creates a minimal schema/table
shape the default scenarios expect. `scenarios/main_test.go` defaults:
- Create temp SQLite DB from the fixture
- Register as a transient vi-sql connection
- Unregister on cleanup

Then `make test-wezterm` Just Works on any clone — no `VI_SQL_WEZTERM_CONNECTION`
required. The current `proxmox` default moves into a `.env.example` for the
maintainer's day-to-day use.

### 4. Debugging: `Pause()` and `Breakpoint()`

```go
s.Pause(5*time.Second)   // explicit wait while watching
s.Breakpoint()           // blocks until any key pressed in the test's terminal
```

`Breakpoint()` in particular is valuable for iterating on a flaky scenario —
run in slow mode, hit a breakpoint, poke the app manually, resume.

### 5. Focus-state MCP tool

For "is the schema tree focused?" style assertions. Small addition in
`internal/mcp/tools.go` that returns the currently focused element ID. Lets us
stop scraping for focus state.

### 6. CI story (or explicit decision not to)

Two honest options:
- **(A) Keep local-only.** Mark in `CONTRIBUTING.md` that `tests/wezterm/` is
  a maintainer tool. Don't advertise it.
- **(B) Publish properly.** Replace wezterm with `tmux` or `go-expect` in CI
  (same harness shape, different backend). Fixture from #3. GitHub Actions
  job runs it headlessly.

Pick one before inviting contributors to add scenarios. Option (A) is fine if
the goal is "fewer regressions for the maintainer." Option (B) is the only
path if the goal is "contributors can trust these tests."

## What needs to happen before this is public-ready

Minimum bar, in priority order:

1. **#3 (portable fixture)** — without this, external contributors can't run
   the tests at all.
2. **#1 (MCP assertions)** — without this, tests are cosmetic-brittle and
   contributors will (rightly) ignore them when they break spuriously.
3. **#6 decision** — either document "maintainer tool" clearly, or get CI
   running. Anything in between rots.

Everything else is polish.

## Not doing

- Generic multi-terminal abstraction. If another contributor wants tmux-based
  scenarios, they get `tests/tmux/` with their own harness. Mixing backends
  behind a common interface is premature — we only have one backend and we
  already know its quirks.
- Scenario DSL (YAML/JSON). Three scenarios isn't enough to justify
  indirection. Reassess if we're past ten.
