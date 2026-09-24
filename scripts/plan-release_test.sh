#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
staged_before="$(git -C "$root" diff --cached --name-status)"
(
  inherited_git_environment=()
  while IFS= read -r variable_name; do
    inherited_git_environment+=("$variable_name")
  done < <(git -C "$root" rev-parse --local-env-vars)
  unset "${inherited_git_environment[@]}"
  git -C "$tmp" init -q
  git -C "$tmp" config core.hooksPath /dev/null
  git -C "$tmp" config user.email test@example.invalid
  git -C "$tmp" config user.name Test
  touch "$tmp/file"
  git -C "$tmp" add file
  git -C "$tmp" commit -qm 'feat: initial capability'
  result="$(cd "$tmp" && "$root/scripts/plan-release.sh")"
  grep -qx 'next_tag=v0.1.0' <<<"$result"
  git -C "$tmp" tag v0.1.0
  echo change >> "$tmp/file"
  git -C "$tmp" commit -qam 'fix: correct behavior'
  result="$(cd "$tmp" && "$root/scripts/plan-release.sh")"
  grep -qx 'next_tag=v0.1.1' <<<"$result"
)
staged_after="$(git -C "$root" diff --cached --name-status)"
[[ "$staged_after" == "$staged_before" ]] || {
  echo "release-plan test mutated the caller's staged index" >&2
  exit 1
}
