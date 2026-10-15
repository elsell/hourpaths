#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
# Compile the pinned Expo engine used by the foreground app, not macOS SQLite.
sqlite_dir="$repo_root/apps/mobile/node_modules/expo-sqlite/vendor/sqlite3"
cp "$sqlite_dir/sqlite3.h" "$work_dir/sqlite3.h"
printf 'module ExpoSQLite { header "sqlite3.h" export * }\n' > "$work_dir/module.modulemap"
clang -c "$sqlite_dir/sqlite3.c" -o "$work_dir/sqlite3.o" \
  -DHAVE_USLEEP=1 -DSQLITE_ENABLE_LOCKING_STYLE=0 -DSQLITE_TEMP_STORE=2
swiftc -I "$work_dir" "$work_dir/sqlite3.o" "$repo_root/apps/mobile/modules/hourpaths-timers/ios/NativeTimerTransaction.swift" \
  "$repo_root/apps/mobile/modules/hourpaths-timers/ios/NativeTimerPresentationCache.swift" \
  "$repo_root/scripts/fixtures/native-timer-transaction.swift" \
  -framework JavaScriptCore -o "$work_dir/native-timer-transaction"
"$work_dir/native-timer-transaction" "$repo_root"

# Exercise the pinned widget renderer's bounded engine reuse and error recovery.
swiftc "$repo_root/apps/mobile/node_modules/expo-widgets/ios/Widgets/WidgetContext.swift" \
  "$repo_root/scripts/fixtures/widget-runtime.swift" \
  -framework JavaScriptCore -o "$work_dir/widget-runtime"
"$work_dir/widget-runtime"
