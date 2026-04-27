#!/bin/sh
set -eu

BIN_NAME="vi-sql"
KEYRING_SERVICE="vi-sql"
KEYRING_ACCOUNT="encryption-key"

err() { printf 'error: %s\n' "$*" >&2; exit 1; }
info() { printf '%s\n' "$*"; }

ask() {
	prompt="$1"
	printf '%s [y/N] ' "$prompt"
	read -r answer
	case "$answer" in
		y|Y|yes|YES) return 0 ;;
		*) return 1 ;;
	esac
}

CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/vi-sql"
LOG_FILE="/tmp/vi-sql.log"

BIN_PATH=$(command -v "$BIN_NAME" 2>/dev/null || true)

info "Vi-SQL uninstall"
info ""

if [ -n "$BIN_PATH" ]; then
	if ask "Remove binary at ${BIN_PATH}?"; then
		if [ -w "$BIN_PATH" ]; then
			rm -f "$BIN_PATH"
		else
			sudo rm -f "$BIN_PATH"
		fi
		info "  removed ${BIN_PATH}"
	fi
else
	info "Binary not found on PATH (skipping)."
fi

if [ -d "$CONFIG_DIR" ]; then
	if ask "Remove config directory ${CONFIG_DIR}?"; then
		rm -rf "$CONFIG_DIR"
		info "  removed ${CONFIG_DIR}"
	fi
fi

if [ -f "$LOG_FILE" ]; then
	if ask "Remove log file ${LOG_FILE}?"; then
		rm -f "$LOG_FILE"
		info "  removed ${LOG_FILE}"
	fi
fi

info ""
info "Keyring entry (only present if you used the 'keyring' security method):"
case "$(uname -s)" in
	Darwin)
		if ask "Remove macOS Keychain entry (${KEYRING_SERVICE}/${KEYRING_ACCOUNT})?"; then
			security delete-generic-password -s "$KEYRING_SERVICE" -a "$KEYRING_ACCOUNT" >/dev/null 2>&1 \
				&& info "  removed Keychain entry" \
				|| info "  no Keychain entry found"
		fi
		;;
	Linux)
		if command -v secret-tool >/dev/null 2>&1; then
			if ask "Remove Secret Service entry (${KEYRING_SERVICE}/${KEYRING_ACCOUNT})?"; then
				secret-tool clear service "$KEYRING_SERVICE" account "$KEYRING_ACCOUNT" \
					&& info "  removed Secret Service entry" \
					|| info "  no Secret Service entry found"
			fi
		else
			info "  secret-tool not installed; remove the entry via your keyring app if you used it"
		fi
		;;
	*)
		info "  remove the OS keyring entry manually if you used the 'keyring' method"
		;;
esac

info ""
info "Done."
