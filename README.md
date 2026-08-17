<div align="center">
  <img src="./assets/logo.svg">
</div>

---

A terminal UI for SQL databases built with passion. Browse schemas, run queries, edit rows, inspect query plans, and expose your session to AI tools via a built-in MCP server.

<a href="./assets/query_autocomplete.png"><img src="./assets/query_autocomplete.png"></a>

<table>
  <tr>
    <td><a href="./assets/actions.png"><img src="./assets/actions.png"></a></td>
    <td><a href="./assets/dracula-style.png"><img src="./assets/dracula-style.png"></a></td>
    <td><a href="./assets/vim-mode.png"><img src="./assets/vim-mode.png"></a></td>
  </tr>
</table>

## Introductory video

[![vi-sql introductory video](https://img.youtube.com/vi/Ver1les7tn8/maxresdefault.jpg)](https://youtu.be/Ver1les7tn8)

## Features

- **Multi-tab SQL editor** — syntax highlighting, autocomplete, query history, and `$EDITOR` integration
- **Table data view** — filter, sort, inline edit, add/delete rows, copy rows as JSON/CSV, follow foreign keys, find references
- **Vim mode** — `hjkl` navigation and multi-key sequences (`gg`, `dd`, `yy`, `yrj`, `yrc`, `gd`, `gr`) across the entire UI
- **Schema browser** — tables, structure, indexes, DDL; create, rename, and drop objects via keybindings
- **EXPLAIN / EXPLAIN ANALYZE** — query plan viewer with cost and timing breakdown
- **Import / Export** — CSV, JSON, SQL INSERT, and Markdown
- **MCP server** — AI assistants (Claude, Cursor, etc.) can browse your schema and draft queries in the editor; query
  execution is opt-in
- **Auto-update** — update to the latest release from inside the app via the actions palette
- **Encrypted connections** — AES-256-GCM encryption; supports OS keyring, master password, or env var
- **Themes** — multiple built-in themes, fully customizable via YAML

## Install

Installing with [cURL](https://curl.se):

```sh
curl -fsSL https://vi-sql.com/install.sh | sh
```

You can also pin a specific version:

```sh
VI_SQL_VERSION=v0.0.3 curl -fsSL https://vi-sql.com/install.sh | sh
```

If you use [Homebrew](https://brew.sh), installation is straightforward:

```sh
brew install vi-sql
```

Precompiled binaries are available on the [releases page](https://github.com/kopecmaciej/vi-sql/releases).

### Building from source

_Requires Go 1.25+_.

```sh
git clone https://github.com/kopecmaciej/vi-sql.git
cd vi-sql
make build
```

### Uninstall

```sh
curl -fsSL https://vi-sql.com/uninstall.sh | sh
```

The script prompts before removing each artifact: the binary, config directory, log file, and any keyring entry.

## Quickstart

Run `vi-sql` and enter your connection details on the welcome screen, or connect directly with a DSN:

```sh
vi-sql --connect postgres://user:pass@localhost/mydb
vi-sql --connect mysql://user:pass@localhost/mydb
vi-sql --connect file:/home/user/data.db
```

Or by saved connection name:

```sh
vi-sql --connection-name mydb
```

Jump straight to a table:

```sh
vi-sql --jump public/users
```

Config and data paths vary by OS. Run `vi-sql --paths` to see the exact locations on your system (Config, Keybindings, Styles, Icons, and Log).

## MCP server

vi-sql ships an HTTP MCP server that AI tools (Claude Code, Cursor, etc.) can connect to while the app is running. Enable it from the options page and point your client at `http://localhost:9741/mcp`.

See the [MCP documentation](https://vi-sql.com/docs/mcp) for setup, available tools, and configuration options.

## Troubleshooting

See the [troubleshooting documentation](https://vi-sql.com/docs/troubleshooting) for common issues and fixes.

## License

Apache 2.0
