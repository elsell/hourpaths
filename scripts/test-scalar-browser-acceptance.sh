#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
makefile="$root/Makefile"
classifier="$root/scripts/ci_changes.py"
browser="$root/scripts/scalar-browser-acceptance.mjs"
retry="$root/scripts/scalar-try-retry.mjs"
live="$root/scripts/live-acceptance.sh"

token_response_line="$(grep -nF 'const tokenResponse = await tokenResponsePromise' "$browser" | cut -d: -f1)"
popup_close_line="$(grep -nF 'for (let attempt = 0; attempt < 50 && !popup.isClosed(); attempt += 1)' "$browser" | cut -d: -f1)"
credential_ready_line="$(grep -nF 'await waitForScalarCredential({' "$browser" | cut -d: -f1)"
try_request_line="$(grep -nF 'const onboarding = await waitForAuthorizedTryRequest' "$browser" | cut -d: -f1)"
test -n "$token_response_line"
test -n "$popup_close_line"
test -n "$credential_ready_line"
test -n "$try_request_line"
test "$token_response_line" -lt "$popup_close_line"
test "$popup_close_line" -lt "$credential_ready_line"
test "$credential_ready_line" -lt "$try_request_line"
grep -Fq "throw new Error('Scalar authorization popup did not close after token exchange')" "$browser"
grep -Fq "throw new Error('Scalar token exchange omitted the application access token')" "$browser"

node --test "$root/scripts/scalar-try-retry.test.mjs"
grep -Fq "from './scalar-try-retry.mjs'" "$browser"
grep -Fq 'waitForScalarCredential,' "$browser"
grep -Fq 'credentialMatches: (expectedToken)' "$browser"
grep -Fq "page.locator('input.scalar-password-input').evaluateAll(" "$browser"
grep -Fq '(inputs, token) => inputs.some((input) => input.value === token)' "$browser"
grep -Fq '}, tokenPayload.access_token, 5_000)' "$browser"
grep -Fq 'stabilityIntervalMs = 500' "$retry"
test "$(grep -Fc 'stableSince = undefined' "$retry")" -eq 2
grep -Fq 'observedAt - stableSince >= stabilityIntervalMs' "$retry"
if grep -Fq "page.locator('input[type=\"password\"]')" "$browser"; then
  echo "Scalar renders password controls as text inputs with the scalar-password-input class" >&2
  exit 1
fi
grep -Fq 'sendVisible: () => sendRequest.isVisible()' "$browser"
grep -Fq 'closeVisible: () => closeClient.isVisible()' "$browser"
grep -Fq 'ensureOpen: (buttonName, timeout) => ensureTryClientOpen(tryUI, buttonName, timeout)' "$browser"
grep -Fq 'reopen: (buttonName, timeout) => reopenTryClient(tryUI, buttonName, timeout)' "$browser"
grep -Fq 'closeClient: (timeout) => closeClient.click({ timeout })' "$browser"
grep -Fq "sendRequest.waitFor({ state: 'hidden', timeout })" "$browser"
grep -Fq "closeClient.waitFor({ state: 'hidden', timeout })" "$browser"
grep -Fq "pathname: '/v1/onboarding'" "$browser"
if grep -Fq "pathname: '/v1/me'" "$browser"; then
  echo "first-sign-in Scalar acceptance must use the restricted onboarding route" >&2
  exit 1
fi
test "$(grep -c 'cause instanceof errors.TimeoutError' "$browser")" -eq 5
if grep -Fq 'closeIfVisible:' "$browser"; then
  echo "Scalar credential refresh must use the explicit bounded reopen contract" >&2
  exit 1
fi
grep -Fq 'cause instanceof errors.TimeoutError' "$browser"
grep -Fq 'node scripts/scalar-browser-acceptance.mjs' "$live"
grep -Fq 'src="/docs/assets/scalar-api-reference-1.44.20.js"' "$live"
grep -Fq 'f349c815d31be09d11e386726da989e3af50c2f1885910764b51f5b0fae9e28e' "$live"
grep -Fq 'hashlib.sha256' "$live"
if grep -Fq 'sha256sum' "$live"; then
  echo "Scalar live verification must remain portable across supported development hosts" >&2
  exit 1
fi
grep -Fq 'scalar-api-reference-LICENSE.txt' "$live"
grep -Fq './scripts/test-scalar-browser-acceptance.sh' "$makefile"
grep -Fq '"scripts/test-scalar-browser-acceptance.sh"' "$classifier"
grep -Fq '"scripts/scalar-try-retry.mjs"' "$classifier"
grep -Fq '"scripts/scalar-try-retry.test.mjs"' "$classifier"

echo "Scalar browser bounded-retry contract test passed"
