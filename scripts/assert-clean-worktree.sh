#!/usr/bin/env bash
set -euo pipefail

readonly status="$(git status --porcelain --untracked-files=all)"
if [[ -n "$status" ]]; then
  echo "verification changed the pinned checkout:" >&2
  echo "$status" >&2
  exit 1
fi
