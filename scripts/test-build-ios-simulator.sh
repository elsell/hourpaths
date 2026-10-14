#!/usr/bin/env bash
set -euo pipefail

readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly subject="$root/scripts/build-ios-simulator.sh"
readonly test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT

mkdir -p "$test_dir/apps/mobile/ios/HourPaths.xcworkspace" "$test_dir/bin"

cat >"$test_dir/bin/xcodebuild" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1 $2" = "-list -json" ]]; then
  printf '%s\n' '{"workspace":{"schemes":["EASClient","Expo","HourPaths","Pods-HourPaths"]}}'
  exit 0
fi
printf '%s\n' "$*" >"$IOS_SCHEME_TEST_ARGS"
FAKE
cat >"$test_dir/bin/bundle" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == 'exec ruby ../../scripts/verify-ios-timer-linking.rb ios' ]]
exit "${IOS_LINKING_TEST_EXIT:-0}"
FAKE
chmod +x "$test_dir/bin/xcodebuild" "$test_dir/bin/bundle"

(
  cd "$test_dir"
  IOS_SCHEME_TEST_ARGS="$test_dir/args" PATH="$test_dir/bin:$PATH" "$subject"
)

grep -Fq -- '-workspace apps/mobile/ios/HourPaths.xcworkspace -scheme HourPaths ' "$test_dir/args"
grep -Fq -- '-destination generic/platform=iOS Simulator' "$test_dir/args"
grep -Fq -- 'ARCHS=arm64 ONLY_ACTIVE_ARCH=YES COMPILER_INDEX_STORE_ENABLE=NO' "$test_dir/args"
if grep -Fq -- '-scheme EASClient ' "$test_dir/args"; then
  echo "iOS simulator build selected a dependency scheme" >&2
  exit 1
fi

rm "$test_dir/args"
if (cd "$test_dir"; IOS_LINKING_TEST_EXIT=1 IOS_SCHEME_TEST_ARGS="$test_dir/args" PATH="$test_dir/bin:$PATH" "$subject"); then
  echo "iOS simulator build ignored missing native timer linkage" >&2
  exit 1
fi
test ! -e "$test_dir/args"
echo "iOS simulator build selects the application scheme and requires timer linkage"
