#!/usr/bin/env bash
set -euo pipefail

readonly root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly remote_gh="$root/scripts/remote-gh.sh"
readonly repository="${HOURPATHS_REPOSITORY:?HOURPATHS_REPOSITORY is required}"

die() {
  printf 'merge refused: %s\n' "$1" >&2
  exit 1
}

if (($# != 2)); then
  echo "usage: $0 PR_NUMBER EXPECTED_HEAD_SHA" >&2
  exit 2
fi

readonly pr_number="$1"
readonly expected_head_sha="$2"

[[ "$pr_number" =~ ^[1-9][0-9]*$ ]] || die "PR_NUMBER must be a positive integer"
[[ "$expected_head_sha" =~ ^[0-9a-f]{40}$ ]] || \
  die "EXPECTED_HEAD_SHA must be a lowercase full 40-character commit SHA"
[[ "$repository" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || \
  die "HOURPATHS_REPOSITORY must use owner/repository format"

policy_json="$(
  "$remote_gh" api \
    "repos/$repository/contents/scripts/merge-checked-pr.sh?ref=main"
)" || die "could not read the trusted merge policy from main"
readonly policy_json

if ! POLICY_JSON="$policy_json" python3 - "${BASH_SOURCE[0]}" <<'PY'
import base64
import binascii
import json
import os
from pathlib import Path
import sys

try:
    response = json.loads(os.environ["POLICY_JSON"])
except (KeyError, json.JSONDecodeError):
    raise SystemExit("trusted merge policy response is not valid JSON")
if not isinstance(response, dict):
    raise SystemExit("trusted merge policy response must be an object")
if response.get("encoding") != "base64" or not isinstance(response.get("content"), str):
    raise SystemExit("trusted merge policy response is incomplete")
try:
    trusted = base64.b64decode(
        "".join(response["content"].splitlines()), validate=True
    )
except (ValueError, binascii.Error):
    raise SystemExit("trusted merge policy content is not valid base64")
if not trusted or trusted != Path(sys.argv[1]).read_bytes():
    raise SystemExit("invoked merge policy does not exactly match main")
PY
then
  die "merge policy provenance did not satisfy the trusted-source policy"
fi

pr_json="$(
  "$remote_gh" pr view "$pr_number" --repo "$repository" \
    --json state,isDraft,headRefOid,baseRefName,mergeStateStatus,changedFiles,author
)" || die "could not read pull request metadata"
readonly pr_json

if ! PR_JSON="$pr_json" python3 - "$expected_head_sha" <<'PY'
import json
import os
import sys

expected_head = sys.argv[1]
try:
    pull_request = json.loads(os.environ["PR_JSON"])
except (KeyError, json.JSONDecodeError):
    raise SystemExit("pull request metadata is not valid JSON")

if not isinstance(pull_request, dict):
    raise SystemExit("pull request metadata must be an object")
required = {
    "state", "isDraft", "headRefOid", "baseRefName", "mergeStateStatus",
    "changedFiles", "author",
}
if not required.issubset(pull_request):
    raise SystemExit("pull request metadata is incomplete")
if pull_request["state"] != "OPEN":
    raise SystemExit("pull request is not open")
if pull_request["isDraft"] is not False:
    raise SystemExit("pull request is a draft or draft state is malformed")
if pull_request["headRefOid"] != expected_head:
    raise SystemExit("pull request head does not match EXPECTED_HEAD_SHA")
if pull_request["baseRefName"] != "main":
    raise SystemExit("pull request base is not main")
if pull_request["mergeStateStatus"] != "CLEAN":
    raise SystemExit("pull request merge state is not clean")
changed_files = pull_request["changedFiles"]
if isinstance(changed_files, bool) or not isinstance(changed_files, int):
    raise SystemExit("pull request changedFiles count is malformed")
if changed_files < 1:
    raise SystemExit("pull request must change at least one file")
author = pull_request["author"]
if (
    not isinstance(author, dict)
    or not isinstance(author.get("login"), str)
    or not author["login"]
):
    raise SystemExit("pull request author is malformed")
PY
then
  die "pull request metadata did not satisfy the immutable merge policy"
fi

files_json="$(
  "$remote_gh" api --paginate --slurp \
    "repos/$repository/pulls/$pr_number/files?per_page=100"
)" || die "could not read complete pull request file list"
readonly files_json

approval_body="$(
  PR_JSON="$pr_json" FILES_JSON="$files_json" python3 - \
    "$repository" "$pr_number" "$expected_head_sha" <<'PY'
import json
import os
import sys

try:
    pages = json.loads(os.environ["FILES_JSON"])
    changed_files = json.loads(os.environ["PR_JSON"])["changedFiles"]
except (KeyError, json.JSONDecodeError):
    raise SystemExit("pull request file results are not valid JSON")

if not isinstance(pages, list) or not pages:
    raise SystemExit("pull request file results must contain API pages")
if any(not isinstance(page, list) or not page for page in pages):
    raise SystemExit("each pull request file API page must be a non-empty array")

files = [file for page in pages for file in page]
if len(files) != changed_files:
    raise SystemExit("pull request file list is incomplete")
protected_paths = {
    "Makefile",
    "apps/mobile/src/policy-link-native.ts",
    "apps/mobile/src/provider-auth-state.ts",
    "apps/mobile/src/provider-auth.ts",
    "apps/mobile/src/provider-discovery.ts",
    "apps/mobile/src/push-notifications-native.ts",
    "apps/web/src/lib/accessibility-focus.ts",
    "apps/web/src/lib/device-locale.ts",
    "apps/web/src/lib/external-policy-link.ts",
    "apps/web/src/lib/notification-convergence-browser.ts",
    "apps/web/src/lib/provider-auth.ts",
    "packages/api-client/package.json",
    "packages/api-client/src/index.ts",
    "packages/client-core/package.json",
    "packages/i18n/package.json",
    "scripts/check-client-api-boundary.mjs",
    "scripts/check-client-api-boundary.py",
    "scripts/check-direct-sql.py",
    "scripts/check-generated-contracts.py",
    "scripts/check_oidc_broker_contract.py",
    "scripts/check-structure.sh",
    "scripts/ci_changes.py",
    "scripts/merge-checked-pr.sh",
    "scripts/protected-api-client-adapter.sha256",
    "scripts/protected-client-capability-adapters.sha256",
    "scripts/protected-provider-adapters.sha256",
    "scripts/remote-gh.sh",
    "scripts/test-generated-contract-drift.py",
    "scripts/test_direct_sql_guard.py",
    "scripts/test_oidc_broker_contract.py",
}
paths = []
reviewed_paths = set()

def is_protected(path):
    return path.startswith(".github/workflows/") or path in protected_paths

for file in files:
    if not isinstance(file, dict) or not isinstance(file.get("filename"), str):
        raise SystemExit("pull request file metadata is malformed")
    path = file["filename"]
    paths.append(path)
    if is_protected(path):
        reviewed_paths.add(path)
    if "previous_filename" in file:
        previous = file["previous_filename"]
        if not isinstance(previous, str) or not previous:
            raise SystemExit("pull request previous filename is malformed")
        if is_protected(previous):
            reviewed_paths.add(previous)
if len(set(paths)) != len(paths):
    raise SystemExit("pull request file list contains duplicate paths")
if reviewed_paths:
    evidence = {
        "repository": sys.argv[1],
        "pullRequest": int(sys.argv[2]),
        "head": sys.argv[3],
        "protectedPaths": sorted(reviewed_paths),
    }
    print(
        "HOURPATHS_PROTECTED_APPROVAL_V1 "
        + json.dumps(evidence, sort_keys=True, separators=(",", ":"))
    )
PY
)" || die "pull request file list did not satisfy the immutable merge policy"
readonly approval_body

if [[ -n "$approval_body" ]]; then
  reviews_json="$(
    "$remote_gh" api --paginate --slurp \
      "repos/$repository/pulls/$pr_number/reviews?per_page=100"
  )" || die "could not read complete protected-path review history"
  readonly reviews_json

  review_approved=false
  if PR_JSON="$pr_json" REVIEWS_JSON="$reviews_json" \
    APPROVAL_BODY="$approval_body" python3 - "$expected_head_sha" <<'PY'
import json
import os
import sys

try:
    pull_request = json.loads(os.environ["PR_JSON"])
    pages = json.loads(os.environ["REVIEWS_JSON"])
except (KeyError, json.JSONDecodeError):
    raise SystemExit("protected-path review results are not valid JSON")
if not isinstance(pages, list) or not pages:
    raise SystemExit("protected-path review results must contain API pages")
if any(not isinstance(page, list) for page in pages):
    raise SystemExit("each protected-path review API page must be an array")

expected_body = os.environ["APPROVAL_BODY"]
expected_head = sys.argv[1]
author = pull_request["author"]["login"].casefold()
trusted_associations = {"OWNER", "MEMBER", "COLLABORATOR"}
latest_decisive = {}
for review in (item for page in pages for item in page):
    if not isinstance(review, dict):
        raise SystemExit("each protected-path review must be an object")
    required = {"user", "state", "commit_id", "body", "author_association"}
    if not required.issubset(review):
        raise SystemExit("a protected-path review is incomplete")
    user = review["user"]
    if (
        not isinstance(user, dict)
        or not isinstance(user.get("login"), str)
        or not user["login"]
    ):
        raise SystemExit("protected-path review author is malformed")
    fields = (
        review["state"], review["commit_id"], review["body"],
        review["author_association"],
    )
    if not all(isinstance(value, str) for value in fields):
        raise SystemExit("protected-path review fields are malformed")
    if review["state"] in {"APPROVED", "CHANGES_REQUESTED", "DISMISSED"}:
        latest_decisive[user["login"].casefold()] = review
approved = any(
    review["state"] == "APPROVED"
    and review["commit_id"] == expected_head
    and review["body"] == expected_body
    and login != author
    and review["author_association"] in trusted_associations
    for login, review in latest_decisive.items()
)
if not approved:
    raise SystemExit(1)
PY
  then
    review_approved=true
  fi

  if [[ "$review_approved" == false ]]; then
    comments_json="$(
      "$remote_gh" api --paginate --slurp \
        "repos/$repository/issues/$pr_number/comments?per_page=100"
    )" || die "could not read complete protected-path owner comment history"
    readonly comments_json

    if ! PR_JSON="$pr_json" COMMENTS_JSON="$comments_json" \
      APPROVAL_BODY="$approval_body" \
      python3 - "$repository" "$pr_number" <<'PY'
import json
import os
import sys

try:
    pages = json.loads(os.environ["COMMENTS_JSON"])
    pull_request = json.loads(os.environ["PR_JSON"])
except (KeyError, json.JSONDecodeError):
    raise SystemExit("protected-path owner comments are not valid JSON")
if not isinstance(pages, list) or not pages:
    raise SystemExit("protected-path owner comments must contain API pages")
if any(not isinstance(page, list) for page in pages):
    raise SystemExit("each protected-path owner comment API page must be an array")

repository = sys.argv[1]
repository_owner = repository.split("/", 1)[0].casefold()
pull_request_author = pull_request["author"]["login"].casefold()
expected_issue_url = f"https://api.github.com/repos/{repository}/issues/{sys.argv[2]}"
expected_body = os.environ["APPROVAL_BODY"].replace(
    "HOURPATHS_PROTECTED_APPROVAL_V1 ",
    "HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 ",
    1,
)
latest_owner_policy_comment = None
for comment in (item for page in pages for item in page):
    if not isinstance(comment, dict):
        raise SystemExit("each protected-path owner comment must be an object")
    required = {"user", "body", "author_association", "performed_via_github_app"}
    if not required.issubset(comment):
        raise SystemExit("a protected-path owner comment is incomplete")
    user = comment["user"]
    if (
        not isinstance(user, dict)
        or not isinstance(user.get("login"), str)
        or not user["login"]
        or user.get("type") != "User"
    ):
        raise SystemExit("protected-path owner comment author is malformed")
    if not isinstance(comment["body"], str) or not isinstance(
        comment["author_association"], str
    ):
        raise SystemExit("protected-path owner comment fields are malformed")
    if comment["performed_via_github_app"] is not None:
        raise SystemExit("protected-path owner comment must be a direct user action")
    if "issue_url" in comment and comment["issue_url"] != expected_issue_url:
        raise SystemExit("protected-path owner comment issue URL is mismatched")
    if (
        comment["author_association"] == "OWNER"
        and comment["body"].startswith("HOURPATHS_PROTECTED_OWNER_")
    ):
        latest_owner_policy_comment = comment
if latest_owner_policy_comment is None:
    raise SystemExit("repository owner attestation is missing")
owner_login = latest_owner_policy_comment["user"]["login"].casefold()
if (
    owner_login != repository_owner
    or owner_login != pull_request_author
    or latest_owner_policy_comment["body"] != expected_body
):
    raise SystemExit(
        "exact repository-owner attestation is required; submit a PR comment "
        f"with this exact body:\n{expected_body}"
    )
PY
    then
      die "protected-path authorization did not satisfy the immutable approval policy"
    fi
  fi
fi

checks_json="$(
  "$remote_gh" pr checks "$pr_number" --repo "$repository" \
    --json name,bucket,state
)" || die "could not read pull request checks"
readonly checks_json

if ! CHECKS_JSON="$checks_json" python3 - <<'PY'
import collections
import json
import os

mandatory = {
    "changes",
    "verify",
    "acceptance",
    "android-native",
    "ios-native",
}
try:
    checks = json.loads(os.environ["CHECKS_JSON"])
except (KeyError, json.JSONDecodeError):
    raise SystemExit("check results are not valid JSON")

if not isinstance(checks, list) or not checks:
    raise SystemExit("check results must be a non-empty array")

names = collections.Counter()
for check in checks:
    if not isinstance(check, dict):
        raise SystemExit("each check result must be an object")
    if set(("name", "bucket", "state")) - check.keys():
        raise SystemExit("a check result is incomplete")
    name = check["name"]
    bucket = check["bucket"]
    state = check["state"]
    if not all(isinstance(value, str) for value in (name, bucket, state)):
        raise SystemExit("check result fields must be strings")
    names[name] += 1
    if bucket.casefold() != "pass":
        raise SystemExit(f"check {name!r} is not passing")
    if state.casefold() not in {"success", "completed"}:
        raise SystemExit(f"check {name!r} is not complete and successful")

invalid_counts = sorted(name for name in mandatory if names[name] != 1)
if invalid_counts:
    raise SystemExit(
        "mandatory checks must appear exactly once: " + ", ".join(invalid_counts)
    )
PY
then
  die "check results did not satisfy the required passing-gates policy"
fi

exec "$remote_gh" pr merge "$pr_number" --repo "$repository" --squash \
  --match-head-commit "$expected_head_sha"
