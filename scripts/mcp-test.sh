#!/usr/bin/env bash
set -euo pipefail

PORT="${MCP_PORT:-9741}"
BASE="http://127.0.0.1:${PORT}/mcp"
SESSION=""
PASS=0
FAIL=0
_TMP=$(mktemp -d)

trap 'rm -rf "$_TMP"' EXIT

# --- session ---

init_session() {
    local headers="$_TMP/init_headers.txt"

    curl -s -D "$headers" -X POST "$BASE" \
        -H "Content-Type: application/json" \
        -H "Accept: application/json, text/event-stream" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"mcp-test","version":"1.0"}}}' \
        > /dev/null &
    local pid=$!

    local i=0
    while [[ $i -lt 50 ]] && ! grep -qi "mcp-session-id" "$headers" 2>/dev/null; do
        sleep 0.1; i=$((i+1))
    done

    SESSION=$(grep -m 1 -i "mcp-session-id" "$headers" | awk '{print $2}' | tr -d '\r' || true)
    kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true

    [[ -n "$SESSION" ]] || { echo "ERROR: MCP session init failed (is vi-sql running with MCP enabled?)" >&2; exit 1; }

    curl -s --max-time 3 -X POST "$BASE" \
        -H "Content-Type: application/json" \
        -H "Mcp-Session-Id: $SESSION" \
        -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null || true
}

# --- tool call ---

call_tool() {
    local name="$1" args="$2"
    local tmp
    tmp=$(mktemp "$_TMP/tool.XXXXXX")

    curl -s -X POST "$BASE" \
        -H "Content-Type: application/json" \
        -H "Accept: application/json, text/event-stream" \
        -H "Mcp-Session-Id: $SESSION" \
        -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"${name}\",\"arguments\":${args}}}" \
        > "$tmp" &
    local pid=$!

    local i=0
    while [[ $i -lt 50 ]] && ! grep -q "^data:" "$tmp" 2>/dev/null; do
        sleep 0.1; i=$((i+1))
    done

    local result
    result=$(grep -m 1 "^data:" "$tmp" | sed 's/^data: //' || true)
    kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true

    echo "$result"
}

# --- assert ---

assert() {
    local label="$1" response="$2"

    if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        echo "  FAIL  $label"
        echo "        $(echo "$response" | jq -r '.error.message')"
        FAIL=$((FAIL+1))
    elif echo "$response" | jq -e '.result' > /dev/null 2>&1; then
        echo "  PASS  $label"
        PASS=$((PASS+1))
    else
        echo "  FAIL  $label (unexpected response)"
        echo "        $response"
        FAIL=$((FAIL+1))
    fi
}

sql_arg() { echo "$1" | jq -Rs '.'; }

# --- tests ---

run_tests() {
    init_session
    echo "Running MCP tests against $BASE"
    echo ""

    assert "get_server_info"    "$(call_tool get_server_info '{}')"
    assert "list_enum_types"    "$(call_tool list_enum_types '{}')"
    assert "execute_query"      "$(call_tool execute_query "{\"query\":$(sql_arg 'SELECT 1 AS n')}")"
    assert "explain_query"      "$(call_tool explain_query "{\"query\":$(sql_arg 'SELECT 1'),\"analyze\":false}")"
    assert "open_query_in_tab"  "$(call_tool open_query_in_tab "{\"query\":$(sql_arg 'SELECT 1 AS test')}")"

    local schemas_resp
    schemas_resp=$(call_tool list_schemas '{"filter":""}')
    assert "list_schemas" "$schemas_resp"

    local schema table
    schema=$(echo "$schemas_resp" | jq -r '.result.content[0].text' | jq -r '.[0].Schema // empty' 2>/dev/null || true)
    table=$(echo  "$schemas_resp" | jq -r '.result.content[0].text' | jq -r '.[0].Tables[0] // empty' 2>/dev/null || true)

    if [[ -n "$schema" && -n "$table" ]]; then
        assert "describe_table ($schema.$table)" \
            "$(call_tool describe_table "{\"schema\":\"$schema\",\"table\":\"$table\"}")"
        assert "sample_table ($schema.$table)" \
            "$(call_tool sample_table "{\"schema\":\"$schema\",\"table\":\"$table\",\"limit\":3}")"
    else
        echo "  SKIP  describe_table (no tables found)"
        echo "  SKIP  sample_table (no tables found)"
    fi

    echo ""
    echo "Results: ${PASS} passed, ${FAIL} failed"
    [[ $FAIL -eq 0 ]]
}

# --- main ---

case "${1:-}" in
    run) run_tests ;;
    call)
        # ./mcp-test.sh call <tool> <json-args>
        # e.g. ./mcp-test.sh call open_query_in_tab '{"query":"SELECT 1"}'
        [[ $# -ge 3 ]] || { echo "usage: $0 call <tool> <json-args>" >&2; exit 1; }
        init_session
        call_tool "$2" "$3" | jq '.'
        ;;
    *)
        echo "MCP test suite for vi-sql"
        echo ""
        echo "  Tests: get_server_info, list_schemas, list_enum_types, execute_query,"
        echo "         explain_query, describe_table, sample_table, open_query_in_tab"
        echo ""
        echo "  Run:        ./mcp-test.sh run"
        echo "  Single:     ./mcp-test.sh call <tool> <json-args>"
        echo "  Example:    ./mcp-test.sh call open_query_in_tab '{\"query\":\"SELECT 1\"}'"
        echo "  Port:       MCP_PORT=${PORT}"
        ;;
esac
