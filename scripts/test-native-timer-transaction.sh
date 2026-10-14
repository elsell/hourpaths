#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
swiftc "$repo_root/apps/mobile/modules/hourpaths-timers/ios/NativeTimerTransaction.swift" \
  "$repo_root/apps/mobile/modules/hourpaths-timers/ios/NativeTimerPresentationCache.swift" \
  "$repo_root/scripts/fixtures/native-timer-transaction.swift" \
  -framework JavaScriptCore -lsqlite3 -o "$work_dir/native-timer-transaction"
"$work_dir/native-timer-transaction" "$repo_root"
