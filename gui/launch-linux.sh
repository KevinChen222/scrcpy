#!/usr/bin/env bash
set -euo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
launcher="$root/build/current-linux/start.sh"
if [[ ! -x "$launcher" ]]; then
    message='找不到 Linux GUI 运行包，请先运行 gui/build-linux.sh。'
    if command -v zenity >/dev/null 2>&1; then
        zenity --error --title='scrcpy LAN' --text="$message" || true
    else
        printf '%s\n' "$message" >&2
    fi
    exit 1
fi
exec "$launcher" "$@"
