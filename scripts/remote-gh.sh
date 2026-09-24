#!/usr/bin/env bash
set -euo pipefail

readonly remote_host="${HOURPATHS_GH_HOST:?HOURPATHS_GH_HOST is required}"

if (($# == 0)); then
  echo "usage: $0 GH_ARGUMENT..." >&2
  exit 2
fi

printf -v remote_arguments ' %q' "$@"
exec ssh "$remote_host" "gh${remote_arguments}"
