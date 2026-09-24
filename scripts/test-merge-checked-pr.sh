#!/usr/bin/env bash
set -euo pipefail

readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly subject="$root/scripts/merge-checked-pr.sh"
readonly valid_sha="0123456789abcdef0123456789abcdef01234567"
readonly test_dir="$(mktemp -d)"
trap 'rm -rf "$test_dir"' EXIT

mkdir -p "$test_dir/bin" "$test_dir/fixtures"

cat >"$test_dir/bin/ssh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >>"$MERGE_CHECKED_CALLS"
case "${2:-}" in
  *"contents/scripts/merge-checked-pr.sh"*) cat "$MERGE_CHECKED_POLICY_FIXTURE" ;;
  *"/reviews"*) cat "$MERGE_CHECKED_REVIEWS_FIXTURE" ;;
  *"/comments"*) cat "$MERGE_CHECKED_COMMENTS_FIXTURE" ;;
  *" pr view "*) cat "$MERGE_CHECKED_VIEW_FIXTURE" ;;
  *" api "*) cat "$MERGE_CHECKED_FILES_FIXTURE" ;;
  *" pr checks "*) cat "$MERGE_CHECKED_CHECKS_FIXTURE" ;;
  *" pr merge "*) printf '%s\n' merged ;;
  *) printf 'unexpected remote command: %s\n' "${2:-}" >&2; exit 97 ;;
esac
EOF
chmod +x "$test_dir/bin/ssh"

python3 - "$subject" "$test_dir/fixtures/policy-valid.json" <<'PY'
import base64
import json
from pathlib import Path
import sys

subject = Path(sys.argv[1])
Path(sys.argv[2]).write_text(
    json.dumps({
        "encoding": "base64",
        "content": base64.b64encode(subject.read_bytes()).decode("ascii"),
    }),
    encoding="utf-8",
)
PY
cat >"$test_dir/fixtures/view-valid.json" <<EOF
{"state":"OPEN","isDraft":false,"headRefOid":"$valid_sha","baseRefName":"main","mergeStateStatus":"CLEAN","changedFiles":1,"author":{"login":"author"}}
EOF
cat >"$test_dir/fixtures/files-valid.json" <<'EOF'
[[{"filename":"apps/api/main.go"}]]
EOF
cat >"$test_dir/fixtures/reviews-empty.json" <<'EOF'
[[]]
EOF
cat >"$test_dir/fixtures/comments-empty.json" <<'EOF'
[[]]
EOF
cat >"$test_dir/fixtures/checks-valid.json" <<'EOF'
[
  {"name":"changes","bucket":"pass","state":"COMPLETED"},
  {"name":"verify","bucket":"pass","state":"COMPLETED"},
  {"name":"acceptance","bucket":"pass","state":"COMPLETED"},
  {"name":"android-native","bucket":"pass","state":"COMPLETED"},
  {"name":"ios-native","bucket":"pass","state":"COMPLETED"}
]
EOF

run_subject() {
  local view_fixture="${1:?view fixture required}"
  local checks_fixture="${2:?checks fixture required}"
  local files_fixture="${MERGE_CHECKED_FILES_FIXTURE:-$test_dir/fixtures/files-valid.json}"
  local policy_fixture="${MERGE_CHECKED_POLICY_FIXTURE:-$test_dir/fixtures/policy-valid.json}"
  local reviews_fixture="${MERGE_CHECKED_REVIEWS_FIXTURE:-$test_dir/fixtures/reviews-empty.json}"
  local comments_fixture="${MERGE_CHECKED_COMMENTS_FIXTURE:-$test_dir/fixtures/comments-empty.json}"
  shift 2
  : >"$test_dir/calls"
  MERGE_CHECKED_CALLS="$test_dir/calls" \
  MERGE_CHECKED_VIEW_FIXTURE="$view_fixture" \
  MERGE_CHECKED_FILES_FIXTURE="$files_fixture" \
  MERGE_CHECKED_POLICY_FIXTURE="$policy_fixture" \
  MERGE_CHECKED_REVIEWS_FIXTURE="$reviews_fixture" \
  MERGE_CHECKED_COMMENTS_FIXTURE="$comments_fixture" \
  MERGE_CHECKED_CHECKS_FIXTURE="$checks_fixture" \
  HOURPATHS_GH_HOST=fixture-host \
  HOURPATHS_REPOSITORY="${HOURPATHS_REPOSITORY:-elsell/hour-paths}" \
  PATH="$test_dir/bin:$PATH" \
    "$subject" "$@"
}

expect_rejected() {
  local description="${1:?description required}"
  shift
  if "$@" >"$test_dir/rejected.out" 2>&1; then
    echo "$description must be rejected" >&2
    exit 1
  fi
}

# Inputs are validated before any SSH boundary is crossed.
for arguments in \
  "not-a-number $valid_sha" \
  "1 short" \
  "1 ${valid_sha}extra"
do
  : >"$test_dir/calls"
  # Intentional word splitting supplies the two invalid CLI arguments.
  # shellcheck disable=SC2086
  expect_rejected "invalid input ($arguments)" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" $arguments
  test ! -s "$test_dir/calls"
done

: >"$test_dir/calls"
expect_rejected "missing arguments" run_subject \
  "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" 42
test ! -s "$test_dir/calls"
: >"$test_dir/calls"
expect_rejected "extra arguments" run_subject \
  "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
  42 "$valid_sha" extra
test ! -s "$test_dir/calls"
: >"$test_dir/calls"
HOURPATHS_REPOSITORY="invalid repository" \
  expect_rejected "invalid repository" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test ! -s "$test_dir/calls"

# A valid, immutable merge executes only from the trusted main policy after all
# read-only gates pass.
run_subject "$test_dir/fixtures/view-valid.json" \
  "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >"$test_dir/valid.out"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
grep -Fq "gh api repos/elsell/hour-paths/contents/scripts/merge-checked-pr.sh\?ref=main" "$test_dir/calls"
grep -Fq "gh pr view 42 --repo elsell/hour-paths --json state\\,isDraft\\,headRefOid\\,baseRefName\\,mergeStateStatus\\,changedFiles\\,author" "$test_dir/calls"
grep -Fq "gh api --paginate --slurp repos/elsell/hour-paths/pulls/42/files\?per_page=100" "$test_dir/calls"
grep -Fq "gh pr checks 42 --repo elsell/hour-paths --json name\\,bucket\\,state" "$test_dir/calls"
grep -Fq "gh pr merge 42 --repo elsell/hour-paths --squash --match-head-commit $valid_sha" "$test_dir/calls"

write_view_fixture() {
  printf '%s\n' "$1" >"$test_dir/fixtures/view-case.json"
}

write_files_fixture() {
  printf '%s\n' "$1" >"$test_dir/fixtures/files-case.json"
}

write_files_fixture '[[{"filename":"packages/api-client/src/index.ts"}]]'
readonly owner_attestation_body="HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[\"packages/api-client/src/index.ts\"],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}"
write_comment_fixture() {
  python3 - "$test_dir/fixtures/comments-case.json" \
    "${1:?body required}" "${2:-elsell}" "${3:-OWNER}" \
    "${4:-User}" "${5:-direct}" \
    "${6:-https://api.github.com/repos/elsell/hour-paths/issues/42}" <<'PY'
import json
from pathlib import Path
import sys

output, body, login, association, user_type, performed, issue_url = sys.argv[1:]
comment = {
    "user": {"login": login, "type": user_type},
    "body": body,
    "author_association": association,
    "performed_via_github_app": None if performed == "direct" else {"id": 1},
    "issue_url": issue_url,
}
Path(output).write_text(json.dumps([[comment]]), encoding="utf-8")
PY
}

# The repository owner may attest their own protected-path pull request with
# exact canonical evidence, including when the comment is on a later API page.
write_view_fixture \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"changedFiles\":1,\"author\":{\"login\":\"elsell\"}}"
write_comment_fixture "$owner_attestation_body"
python3 - "$test_dir/fixtures/comments-case.json" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1])
attestation = json.loads(path.read_text(encoding="utf-8"))[0][0]
path.write_text(json.dumps([[
    {"user": {"login": "observer", "type": "User"}, "body": "reviewing", "author_association": "MEMBER", "performed_via_github_app": None}
], [attestation]]), encoding="utf-8")
PY
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_COMMENTS_FIXTURE="$test_dir/fixtures/comments-case.json" \
  run_subject "$test_dir/fixtures/view-case.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 7
grep -Fq "gh api --paginate --slurp repos/elsell/hour-paths/issues/42/comments\?per_page=100" "$test_dir/calls"

# Owner evidence must remain exact and fail closed for stale, malformed,
# non-owner, wrong-association, wrong-PR, or incomplete-path attestations.
for comment_case in \
  "stale head|HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 {\"head\":\"ffffffffffffffffffffffffffffffffffffffff\",\"protectedPaths\":[\"packages/api-client/src/index.ts\"],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}|elsell|OWNER" \
  "malformed body|HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 not-json|elsell|OWNER" \
  "wrong author|$owner_attestation_body|someone-else|OWNER" \
  "wrong association|$owner_attestation_body|elsell|MEMBER" \
  "wrong pull request|HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[\"packages/api-client/src/index.ts\"],\"pullRequest\":41,\"repository\":\"elsell/hour-paths\"}|elsell|OWNER" \
  "incomplete path set|HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}|elsell|OWNER"
do
  IFS='|' read -r description body login association <<<"$comment_case"
  write_comment_fixture "$body" "$login" "$association"
  MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
  MERGE_CHECKED_COMMENTS_FIXTURE="$test_dir/fixtures/comments-case.json" \
    expect_rejected "$description owner attestation" run_subject \
      "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
done

for identity_case in \
  "bot author|Bot|direct|https://api.github.com/repos/elsell/hour-paths/issues/42" \
  "app author|User|app|https://api.github.com/repos/elsell/hour-paths/issues/42" \
  "wrong issue|User|direct|https://api.github.com/repos/elsell/hour-paths/issues/41"
do
  IFS='|' read -r description user_type performed issue_url <<<"$identity_case"
  write_comment_fixture "$owner_attestation_body" elsell OWNER \
    "$user_type" "$performed" "$issue_url"
  MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
  MERGE_CHECKED_COMMENTS_FIXTURE="$test_dir/fixtures/comments-case.json" \
    expect_rejected "$description owner attestation" run_subject \
      "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
done

# A later policy-prefixed owner comment supersedes and revokes an earlier exact
# attestation when its evidence is malformed or noncanonical.
write_comment_fixture "$owner_attestation_body"
python3 - "$test_dir/fixtures/comments-case.json" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1])
attestation = json.loads(path.read_text(encoding="utf-8"))[0][0]
revocation = dict(attestation)
revocation["body"] = "HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 not-json"
path.write_text(json.dumps([[attestation, revocation]]), encoding="utf-8")
PY
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_COMMENTS_FIXTURE="$test_dir/fixtures/comments-case.json" \
  expect_rejected "superseded owner attestation" run_subject \
    "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5

printf '%s\n' '[[{"user":null,"body":"x","author_association":"OWNER"}]]' \
  >"$test_dir/fixtures/comments-malformed.json"
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_COMMENTS_FIXTURE="$test_dir/fixtures/comments-malformed.json" \
  expect_rejected "malformed owner comment history" run_subject \
    "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5

MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_COMMENTS_FIXTURE="$test_dir/fixtures/does-not-exist.json" \
  expect_rejected "owner comment history lookup failure" run_subject \
    "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5

write_view_fixture \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"changedFiles\":1,\"author\":{\"login\":\"author\"}}"

# The executable policy must exactly match the version fetched from main.
printf '%s\n' '{"encoding":"base64","content":"dGFtcGVyZWQK"}' \
  >"$test_dir/fixtures/policy-tampered.json"
MERGE_CHECKED_POLICY_FIXTURE="$test_dir/fixtures/policy-tampered.json" \
  expect_rejected "policy not sourced from main" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 1
for malformed_policy in \
  '' \
  'not-json' \
  '[]' \
  '{}' \
  '{"encoding":"utf-8","content":"dGVzdA=="}' \
  '{"encoding":"base64","content":17}' \
  '{"encoding":"base64","content":"%%%"}' \
  '{"encoding":"base64","content":""}'
do
  printf '%s\n' "$malformed_policy" >"$test_dir/fixtures/policy-malformed.json"
  MERGE_CHECKED_POLICY_FIXTURE="$test_dir/fixtures/policy-malformed.json" \
    expect_rejected "malformed trusted policy ($malformed_policy)" run_subject \
      "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 1
done

# Repository selection is explicit and safely transported by remote-gh.sh.
HOURPATHS_REPOSITORY="owner/custom-repo" \
  run_subject "$test_dir/fixtures/view-valid.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
grep -Fq -- "--repo owner/custom-repo" "$test_dir/calls"

# Remote command failures fail closed and never advance to a merge.
MERGE_CHECKED_POLICY_FIXTURE="$test_dir/fixtures/does-not-exist.json" \
  expect_rejected "trusted policy lookup failure" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 1
expect_rejected "pull request lookup failure" run_subject \
  "$test_dir/fixtures/does-not-exist.json" "$test_dir/fixtures/checks-valid.json" \
  42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 2
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/does-not-exist.json" \
  expect_rejected "pull request files lookup failure" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 3
expect_rejected "checks lookup failure" run_subject \
  "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/does-not-exist.json" \
  42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 4

write_view_fixture() {
  printf '%s\n' "$1" >"$test_dir/fixtures/view-case.json"
}

# More than 100 changed files are accepted only when every API page is present.
python3 - "$test_dir/fixtures" <<'PY'
import json
from pathlib import Path
import sys

fixtures = Path(sys.argv[1])
first_page = [
    {"filename": f"apps/api/file-{index:03}.go"} for index in range(100)
]
(fixtures / "files-paginated.json").write_text(
    json.dumps([first_page, [{"filename": "apps/api/file-100.go"}]]),
    encoding="utf-8",
)
(fixtures / "files-incomplete.json").write_text(
    json.dumps([first_page]), encoding="utf-8"
)
(fixtures / "files-protected-second-page.json").write_text(
    json.dumps([first_page, [{"filename": "scripts/merge-checked-pr.sh"}]]),
    encoding="utf-8",
)
PY
write_view_fixture \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"changedFiles\":101,\"author\":{\"login\":\"author\"}}"
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-paginated.json" \
  run_subject "$test_dir/fixtures/view-case.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-incomplete.json" \
  expect_rejected "incomplete pagination" run_subject \
    "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 3
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-protected-second-page.json" \
  expect_rejected "protected file on a later page" run_subject \
    "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5

for invalid_view in \
  '' \
  'not-json' \
  '[]' \
  "{\"state\":\"CLOSED\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"mergeStateStatus\":\"CLEAN\"}" \
  "{\"state\":\"OPEN\",\"isDraft\":true,\"headRefOid\":\"$valid_sha\",\"mergeStateStatus\":\"CLEAN\"}" \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"ffffffffffffffffffffffffffffffffffffffff\",\"mergeStateStatus\":\"CLEAN\"}" \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"baseRefName\":\"develop\",\"mergeStateStatus\":\"CLEAN\",\"changedFiles\":1}" \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"mergeStateStatus\":\"BLOCKED\"}" \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\"}"
do
  write_view_fixture "$invalid_view"
  expect_rejected "unsafe pull request metadata ($invalid_view)" run_subject \
    "$test_dir/fixtures/view-case.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 2
done

write_files_fixture() {
  printf '%s\n' "$1" >"$test_dir/fixtures/files-case.json"
}

for invalid_files in \
  '' \
  'not-json' \
  '{}' \
  '[]' \
  '[[]]' \
  '[[{"filename":"apps/api/main.go"}],[]]' \
  '[[{"path":"apps/api/main.go"}]]' \
  '[[{"filename":"apps/api/main.go"},{"filename":"apps/api/extra.go"}]]'
do
  write_files_fixture "$invalid_files"
  MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
    expect_rejected "unsafe pull request files ($invalid_files)" run_subject \
      "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 3
done

# Every protected path requires exact independent approval, including workflow
# files and protection artifacts on the second page.
for protected_files in \
  '[[{"filename":".github/workflows/ci.yml"}]]' \
  '[[{"filename":"Makefile"}]]' \
  '[[{"filename":"packages/api-client/src/index.ts"}]]' \
  '[[{"filename":"packages/api-client/package.json"}]]' \
  '[[{"filename":"packages/client-core/package.json"}]]' \
  '[[{"filename":"packages/i18n/package.json"}]]' \
  '[[{"filename":"apps/mobile/src/policy-link-native.ts"}]]' \
  '[[{"filename":"apps/mobile/src/provider-auth-state.ts"}]]' \
  '[[{"filename":"apps/mobile/src/provider-auth.ts"}]]' \
  '[[{"filename":"apps/mobile/src/provider-discovery.ts"}]]' \
  '[[{"filename":"apps/mobile/src/push-notifications-native.ts"}]]' \
  '[[{"filename":"apps/web/src/lib/accessibility-focus.ts"}]]' \
  '[[{"filename":"apps/web/src/lib/device-locale.ts"}]]' \
  '[[{"filename":"apps/web/src/lib/external-policy-link.ts"}]]' \
  '[[{"filename":"apps/web/src/lib/notification-convergence-browser.ts"}]]' \
  '[[{"filename":"apps/web/src/lib/provider-auth.ts"}]]' \
  '[[{"filename":"scripts/check-client-api-boundary.mjs"}]]' \
  '[[{"filename":"scripts/check-client-api-boundary.py"}]]' \
  '[[{"filename":"scripts/check-direct-sql.py"}]]' \
  '[[{"filename":"scripts/check_oidc_broker_contract.py"}]]' \
  '[[{"filename":"scripts/check-structure.sh"}]]' \
  '[[{"filename":"scripts/check-generated-contracts.py"}]]' \
  '[[{"filename":"scripts/ci_changes.py"}]]' \
  '[[{"filename":"scripts/merge-checked-pr.sh"}]]' \
  '[[{"filename":"scripts/protected-api-client-adapter.sha256"}]]' \
  '[[{"filename":"scripts/protected-client-capability-adapters.sha256"}]]' \
  '[[{"filename":"scripts/protected-provider-adapters.sha256"}]]' \
  '[[{"filename":"scripts/remote-gh.sh"}]]' \
  '[[{"filename":"scripts/test-generated-contract-drift.py"}]]' \
  '[[{"filename":"scripts/test_direct_sql_guard.py"}]]' \
  '[[{"filename":"scripts/test_oidc_broker_contract.py"}]]'
do
  write_files_fixture "$protected_files"
  MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
    expect_rejected "unapproved protected files ($protected_files)" run_subject \
      "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
done

readonly approval_body="HOURPATHS_PROTECTED_APPROVAL_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[\"packages/api-client/src/index.ts\"],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}"
write_review_fixture() {
  python3 - "$test_dir/fixtures/reviews-case.json" \
    "${1:?body required}" "${2:-APPROVED}" "${3:-$valid_sha}" \
    "${4:-reviewer}" "${5:-COLLABORATOR}" <<'PY'
import json
from pathlib import Path
import sys

output, body, state, commit_id, login, association = sys.argv[1:]
review = {
    "user": {"login": login},
    "state": state,
    "commit_id": commit_id,
    "body": body,
    "author_association": association,
}
Path(output).write_text(json.dumps([[review]]), encoding="utf-8")
PY
}

# A collaborator's exact canonical approval of this repository, PR, head, and
# complete protected-path set permits the normal five-check exact-head merge.
write_files_fixture '[[{"filename":"packages/api-client/src/index.ts"}]]'
write_review_fixture "$approval_body"
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-case.json" \
  run_subject "$test_dir/fixtures/view-valid.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 6
grep -Fq "gh api --paginate --slurp repos/elsell/hour-paths/pulls/42/reviews\?per_page=100" "$test_dir/calls"
grep -Fq "gh pr checks 42 --repo elsell/hour-paths --json name\\,bucket\\,state" "$test_dir/calls"
grep -Fq "gh pr merge 42 --repo elsell/hour-paths --squash --match-head-commit $valid_sha" "$test_dir/calls"

# Canonical evidence sorts and binds the complete protected set, and an
# approval found on a later review page remains authoritative.
readonly multi_approval_body="HOURPATHS_PROTECTED_APPROVAL_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[\"apps/mobile/src/provider-auth.ts\",\"packages/api-client/src/index.ts\"],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}"
write_view_fixture \
  "{\"state\":\"OPEN\",\"isDraft\":false,\"headRefOid\":\"$valid_sha\",\"baseRefName\":\"main\",\"mergeStateStatus\":\"CLEAN\",\"changedFiles\":2,\"author\":{\"login\":\"author\"}}"
write_files_fixture '[[{"filename":"packages/api-client/src/index.ts"},{"filename":"apps/mobile/src/provider-auth.ts"}]]'
write_review_fixture "$multi_approval_body"
python3 - "$test_dir/fixtures/reviews-case.json" <<'PY'
import json
from pathlib import Path
import sys

path = Path(sys.argv[1])
approval = json.loads(path.read_text(encoding="utf-8"))[0][0]
path.write_text(json.dumps([[
    {
        "user": {"login": "observer"}, "state": "COMMENTED",
        "commit_id": approval["commit_id"], "body": "reviewing",
        "author_association": "MEMBER",
    },
], [approval]]), encoding="utf-8")
PY
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-case.json" \
  run_subject "$test_dir/fixtures/view-case.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 6

# Rename metadata is part of protection classification so moving a protected
# artifact to an ordinary-looking path cannot evade independent approval.
readonly rename_approval_body="HOURPATHS_PROTECTED_APPROVAL_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[\"packages/api-client/src/index.ts\"],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}"
write_files_fixture '[[{"filename":"packages/api-client/src/renamed.ts","previous_filename":"packages/api-client/src/index.ts"}]]'
write_review_fixture "$rename_approval_body"
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-case.json" \
  run_subject "$test_dir/fixtures/view-valid.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 6

# A rename between two protected locations binds both names exactly once.
readonly double_rename_body="HOURPATHS_PROTECTED_APPROVAL_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[\"apps/mobile/src/provider-auth.ts\",\"packages/api-client/src/index.ts\"],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}"
write_files_fixture '[[{"filename":"apps/mobile/src/provider-auth.ts","previous_filename":"packages/api-client/src/index.ts"}]]'
write_review_fixture "$double_rename_body"
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-case.json" \
  run_subject "$test_dir/fixtures/view-valid.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 6

# Approval evidence fails closed when absent, stale, self-authored, untrusted,
# non-approved, malformed, noncanonical, or incomplete.
write_files_fixture '[[{"filename":"packages/api-client/src/index.ts"}]]'
for review_case in \
  "stale head|$approval_body|APPROVED|ffffffffffffffffffffffffffffffffffffffff|reviewer|COLLABORATOR" \
  "self-authored|$approval_body|APPROVED|$valid_sha|author|OWNER" \
  "case-variant self-authored|$approval_body|APPROVED|$valid_sha|AuThOr|OWNER" \
  "untrusted reviewer|$approval_body|APPROVED|$valid_sha|reviewer|CONTRIBUTOR" \
  "comment only|$approval_body|COMMENTED|$valid_sha|reviewer|COLLABORATOR" \
  "changes requested|$approval_body|CHANGES_REQUESTED|$valid_sha|reviewer|COLLABORATOR" \
  "dismissed|$approval_body|DISMISSED|$valid_sha|reviewer|COLLABORATOR" \
  "malformed body|not-json|APPROVED|$valid_sha|reviewer|COLLABORATOR" \
  "noncanonical body|$approval_body |APPROVED|$valid_sha|reviewer|COLLABORATOR" \
  "incomplete path set|HOURPATHS_PROTECTED_APPROVAL_V1 {\"head\":\"$valid_sha\",\"protectedPaths\":[],\"pullRequest\":42,\"repository\":\"elsell/hour-paths\"}|APPROVED|$valid_sha|reviewer|COLLABORATOR"
do
  IFS='|' read -r description body state commit_id login association \
    <<<"$review_case"
  write_review_fixture "$body" "$state" "$commit_id" "$login" "$association"
  MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
  MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-case.json" \
    expect_rejected "$description" run_subject \
      "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
done

for malformed_reviews in \
  '' \
  'not-json' \
  '{}' \
  '[]' \
  '[[null]]' \
  '[[{"user":{"login":"reviewer"},"state":"APPROVED"}]]' \
  '[[{"user":null,"state":"APPROVED","commit_id":"x","body":"x","author_association":"OWNER"}]]'
do
  printf '%s\n' "$malformed_reviews" >"$test_dir/fixtures/reviews-malformed.json"
  MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
  MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-malformed.json" \
    expect_rejected "malformed review history ($malformed_reviews)" run_subject \
      "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
      42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5
done

MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/does-not-exist.json" \
  expect_rejected "review history lookup failure" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 4

# A later decisive review by the same reviewer supersedes an earlier approval.
python3 - "$test_dir/fixtures/reviews-superseded.json" "$approval_body" \
  "$valid_sha" <<'PY'
import json
from pathlib import Path
import sys

def review(state, body):
    return {
        "user": {"login": "reviewer"}, "state": state,
        "commit_id": sys.argv[3], "body": body,
        "author_association": "COLLABORATOR",
    }

Path(sys.argv[1]).write_text(json.dumps([[
    review("APPROVED", sys.argv[2]),
    review("CHANGES_REQUESTED", "found a release risk"),
]]), encoding="utf-8")
PY
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-superseded.json" \
  expect_rejected "superseded protected approval" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5

# A later exact approval restores authority after a change request, while a
# non-decisive comment does not revoke the latest approval.
python3 - "$test_dir/fixtures/reviews-restored.json" "$approval_body" \
  "$valid_sha" <<'PY'
import json
from pathlib import Path
import sys

def review(state, body):
    return {
        "user": {"login": "reviewer"}, "state": state,
        "commit_id": sys.argv[3], "body": body,
        "author_association": "COLLABORATOR",
    }

Path(sys.argv[1]).write_text(json.dumps([[
    review("CHANGES_REQUESTED", "fix this"),
    review("APPROVED", sys.argv[2]),
    review("COMMENTED", "follow-up note"),
]]), encoding="utf-8")
PY
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-restored.json" \
  run_subject "$test_dir/fixtures/view-valid.json" \
    "$test_dir/fixtures/checks-valid.json" 42 "$valid_sha" >/dev/null
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 6

# Dismissal is decisive and revokes an earlier exact approval.
python3 - "$test_dir/fixtures/reviews-dismissed.json" "$approval_body" \
  "$valid_sha" <<'PY'
import json
from pathlib import Path
import sys

def review(state, body):
    return {
        "user": {"login": "reviewer"}, "state": state,
        "commit_id": sys.argv[3], "body": body,
        "author_association": "COLLABORATOR",
    }

Path(sys.argv[1]).write_text(json.dumps([[
    review("APPROVED", sys.argv[2]),
    review("DISMISSED", sys.argv[2]),
]]), encoding="utf-8")
PY
MERGE_CHECKED_FILES_FIXTURE="$test_dir/fixtures/files-case.json" \
MERGE_CHECKED_REVIEWS_FIXTURE="$test_dir/fixtures/reviews-dismissed.json" \
  expect_rejected "dismissed exact approval" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-valid.json" \
    42 "$valid_sha"
test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 5

write_checks_fixture() {
  printf '%s\n' "$1" >"$test_dir/fixtures/checks-case.json"
}

for invalid_checks in \
  '' \
  'not-json' \
  '{}' \
  '[]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"}]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"},{"name":"verify","bucket":"pass","state":"COMPLETED"},{"name":"acceptance","bucket":"pass","state":"COMPLETED"},{"name":"android-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"pass","state":"COMPLETED"}]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"},{"name":"verify","bucket":"pending","state":"IN_PROGRESS"},{"name":"acceptance","bucket":"pass","state":"COMPLETED"},{"name":"android-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"pass","state":"COMPLETED"}]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"},{"name":"verify","bucket":"fail","state":"COMPLETED"},{"name":"acceptance","bucket":"pass","state":"COMPLETED"},{"name":"android-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"pass","state":"COMPLETED"}]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"},{"name":"verify","bucket":"pass","state":"COMPLETED"},{"name":"acceptance","bucket":"pass","state":"COMPLETED"},{"name":"android-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"skipping","state":"COMPLETED"}]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"},{"name":"verify","bucket":"pass","state":"COMPLETED"},{"name":"acceptance","bucket":"pass","state":"COMPLETED"},{"name":"android-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"pass","state":"COMPLETED"},{"name":"unexpected","bucket":"fail","state":"COMPLETED"}]' \
  '[{"name":"changes","bucket":"pass","state":"COMPLETED"},{"name":"verify","bucket":"pass","state":"COMPLETED"},{"name":"acceptance","bucket":"pass","state":"COMPLETED"},{"name":"android-native","bucket":"pass","state":"COMPLETED"},{"name":"ios-native","bucket":"pass","state":"CANCELLED"}]'
do
  write_checks_fixture "$invalid_checks"
  expect_rejected "unsafe check result ($invalid_checks)" run_subject \
    "$test_dir/fixtures/view-valid.json" "$test_dir/fixtures/checks-case.json" \
    42 "$valid_sha"
  test "$(wc -l <"$test_dir/calls" | tr -d ' ')" -eq 4
done

echo "checked merges require immutable clean pull requests and complete unique passing gates"
