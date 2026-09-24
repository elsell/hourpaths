#!/usr/bin/env bash
set -euo pipefail
workspace="$(find apps/mobile/ios -maxdepth 1 -name '*.xcworkspace' -print -quit)"
test -n "$workspace" || { echo "iOS workspace missing; run make mobile-prebuild" >&2; exit 1; }
application_scheme="$(basename "$workspace" .xcworkspace)"
scheme="$(xcodebuild -list -json -workspace "$workspace" | python3 -c 'import json,sys; data=json.load(sys.stdin); expected=sys.argv[1]; schemes=data.get("workspace",{}).get("schemes",[]); sys.stdout.write(expected if expected in schemes else "")' "$application_scheme")"
test -n "$scheme" || { echo "iOS application scheme '$application_scheme' missing" >&2; exit 1; }
xcodebuild -workspace "$workspace" -scheme "$scheme" -configuration Debug -sdk iphonesimulator \
  -destination 'generic/platform=iOS Simulator' \
  ARCHS=arm64 ONLY_ACTIVE_ARCH=YES COMPILER_INDEX_STORE_ENABLE=NO CODE_SIGNING_ALLOWED=NO build
