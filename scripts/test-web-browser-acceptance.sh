#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
subject="$root/scripts/web-browser-acceptance.mjs"
empty_home="$root/scripts/web-empty-home-acceptance.mjs"
manual_activity="$root/scripts/web-manual-activity-acceptance.mjs"
goal_update="$root/scripts/web-goal-update-acceptance.mjs"
notification_convergence="$root/scripts/web-notification-convergence-acceptance.mjs"
path_visibility="$root/scripts/web-path-visibility-acceptance.mjs"
live="$root/scripts/live-acceptance.sh"
notification_spec="$root/specs/notifications/notifications.spec.md"
initial_acceptance="$root/specs/acceptance/initial-product.scenarios.spec.md"
makefile="$root/Makefile"
classifier="$root/scripts/ci_changes.py"

test -f "$subject"
test -f "$empty_home"
test -f "$manual_activity"
test -f "$goal_update"
test -f "$notification_convergence"
test -f "$path_visibility"
test -f "$notification_spec"
test -f "$initial_acceptance"
node --test "$root/scripts/web-browser-navigation.test.mjs"
grep -Fq "const first = await openApplicationPage(context)" "$notification_convergence"
grep -Fq "const second = await openApplicationPage(context)" "$notification_convergence"
grep -Fq "await first.page.bringToFront()" "$notification_convergence"
grep -Fq '/v1/path-invitations?limit=25' "$notification_convergence"
grep -Fq 'node scripts/web-notification-convergence-acceptance.mjs' "$live"
grep -Fq 'node scripts/web-path-visibility-acceptance.mjs' "$live"
grep -Fq 'Manager-removal acceptance slice (`NOTE-03A`)' "$notification_spec"
grep -Fq '### NOTE-03A: Remove a stale Path nudge after manager removal' "$initial_acceptance"
grep -Fq 'NOTE_03A_MANAGER_REMOVAL' "$live"
grep -Fq 'note-03a-unrelated-path-create-key-01' "$live"
grep -Fq 'note-03a-unrelated-invitation-key-01' "$live"
grep -Fq 'note-03a-target-path-create-key-01' "$live"
grep -Fq 'note-03a-target-invitation-key-01' "$live"
grep -Fq 'note-03a-target-accept-key-0001' "$live"
grep -Fq 'note-03a-nudge-key-000000001' "$live"
grep -Fq 'docker compose stop push-provider' "$live"
grep -Fq '"http://localhost:8080/v1/paths/$note_03a_target_path_id/members/$supporter_id/nudges"' "$live"
grep -Fq 'note-03a-member-removal-key-001' "$live"
grep -Fq -- '--data '\''{"confirmed":true,"expectedRole":"participant"}'\''' "$live"
grep -Fq '"activityDeleted":True' "$live"
grep -Fq 'note_03a_removed_nudge_response' "$live"
grep -Fq 'note_03a_missing_notification_response' "$live"
grep -Fq '"$note_03a_removed_nudge_response" == "$note_03a_missing_notification_response"' "$live"
grep -Fq 'item["type"]=="path_member_removed"' "$live"
grep -Fq 'item["id"]==os.environ["UNRELATED_NOTIFICATION_ID"]' "$live"
grep -Fq 'd["meta"]["unreadCount"]==int(os.environ["EXPECTED_UNREAD_COUNT"])' "$live"
grep -Fq 'docker compose start push-provider' "$live"
grep -Fq "'{{.State.Health.Status}}'" "$live"
grep -Fq "failure_code = 'network'" "$live"
grep -Fq 'available_at > CURRENT_TIMESTAMP' "$live"
grep -Fq 'notification_push_delivery_models' "$live"
grep -Fq 'NOTE-03A manager removal stale-nudge cleanup, unread convergence, opaque lookup, unrelated retention, and queued-push suppression acceptance passed' "$live"
grep -Fq 'Voluntary-leave acceptance slice (`NOTE-03B`)' "$notification_spec"
grep -Fq '### NOTE-03B: Remove a stale Path nudge after voluntary leave' "$initial_acceptance"
grep -Fq 'NOTE_03B_VOLUNTARY_LEAVE' "$live"
grep -Fq 'note-03b-target-path-create-key-01' "$live"
grep -Fq 'note-03b-target-invitation-key-01' "$live"
grep -Fq 'note-03b-target-accept-key-0001' "$live"
grep -Fq 'note-03b-nudge-key-000000001' "$live"
grep -Fq 'note-03b-leave-key-000000001' "$live"
grep -Fq '"http://localhost:8080/v1/paths/$note_03b_target_path_id/membership"' "$live"
grep -Fq -- '--data '\''{"confirmed":true,"retainActivity":true}'\''' "$live"
grep -Fq 'item["type"]=="path_member_left"' "$live"
grep -Fq 'note_03b_removed_nudge_response' "$live"
grep -Fq 'note_03b_missing_notification_response' "$live"
grep -Fq '"$note_03b_removed_nudge_response" == "$note_03b_missing_notification_response"' "$live"
grep -Fq '"$note_03b_unrelated_after" == "$note_03b_unrelated_before"' "$live"
grep -Fq 'note_03b_manager_notification_id' "$live"
grep -Fq '"$note_03b_persistence" == '\''0|0|1|1|t'\''' "$live"
grep -Fq 'PUSH_NOTIFICATION_ID="$note_03b_nudge_notification_id"' "$live"
grep -Fq 'NOTE-03B voluntary leave stale-nudge cleanup, unread convergence, opaque lookup, unrelated and manager-notice retention, and queued-push suppression acceptance passed' "$live"
grep -Fq 'Retained-activity interaction acceptance slice (`NOTE-03C`)' "$notification_spec"
grep -Fq '### NOTE-03C: Remove a hidden retained-activity interaction notice' "$initial_acceptance"
grep -Fq 'NOTE_03C_HIDDEN_RETAINED_ACTIVITY' "$live"
grep -Fq 'note-03c-activity-create-key-0001' "$live"
grep -Fq 'note-03c-comment-create-key-00001' "$live"
grep -Fq 'note-03c-leave-key-000000001' "$live"
grep -Fq 'note_03c_interaction_notification_id' "$live"
grep -Fq 'note_03c_removed_interaction_response' "$live"
grep -Fq '"$note_03c_removed_interaction_response" == "$note_03c_missing_notification_response"' "$live"
grep -Fq '"$note_03c_unrelated_after" == "$note_03c_unrelated_before"' "$live"
grep -Fq 'note_03c_manager_notification_id' "$live"
grep -Fq 'PUSH_NOTIFICATION_ID="$note_03c_interaction_notification_id"' "$live"
grep -Fq 'NOTE_03C_REJOIN_MEMBERSHIP' "$live"
grep -Fq 'NOTE-03C retained-activity interaction cleanup, unread convergence, opaque lookup, unrelated and manager-notice retention, push suppression, and non-resurrection acceptance passed' "$live"
grep -Fq 'Visibility-contraction interaction acceptance slice (`NOTE-03D`)' "$notification_spec"
grep -Fq '### NOTE-03D: Remove an inaccessible interaction notice after visibility contraction' "$initial_acceptance"
grep -Fq 'NOTE_03D_VISIBILITY_CONTRACTION_CLEANUP' "$live"
grep -Fq 'note-03d-interaction-path-create-key-01' "$live"
grep -Fq 'note-03d-comment-create-key-00001' "$live"
grep -Fq 'note-03d-comment-heart-key-000001' "$live"
grep -Fq 'note-03d-visibility-private-key-01' "$live"
grep -Fq 'note-03d-visibility-public-key-001' "$live"
grep -Fq 'note_03d_interaction_notification_id' "$live"
grep -Fq 'NOTE_03D_INTERACTION_HISTORY_READY' "$live"
grep -Fq 'note_03d_removed_interaction_response' "$live"
grep -Fq '"$note_03d_removed_interaction_response" == "$note_03d_missing_notification_response"' "$live"
grep -Fq '"$note_03d_unrelated_after" == "$note_03d_unrelated_before"' "$live"
grep -Fq 'PUSH_NOTIFICATION_ID="$note_03d_interaction_notification_id"' "$live"
grep -Fq "failure_code = 'path_access_revoked'" "$live"
grep -Fq 'token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL' "$live"
grep -Fq 'NOTE-03D visibility-contraction interaction cleanup, unread convergence, opaque lookup, unrelated retention, push suppression, and non-resurrection acceptance passed' "$live"
python3 - "$live" <<'PY'
import pathlib
import sys

source = pathlib.Path(sys.argv[1]).read_text()
note_03a = source.index("# NOTE_03A_MANAGER_REMOVAL")
target_path = source.index("note-03a-target-path-create-key-01", note_03a)
target_invitation = source.index("note-03a-target-invitation-key-01", target_path)
target_acceptance = source.index("note-03a-target-accept-key-0001", target_invitation)
provider_pause = source.index("docker compose stop push-provider", target_acceptance)
target_nudge = source.index('"http://localhost:8080/v1/paths/$note_03a_target_path_id/members/$supporter_id/nudges"', provider_pause)
retry_queued = source.index("note_03a_nudge_retry_queued", target_nudge)
provider_resume = source.index("docker compose start push-provider", retry_queued)
member_removal = source.index("note-03a-member-removal-key-001", provider_resume)
assert note_03a < target_path < target_invitation < target_acceptance < provider_pause < target_nudge < retry_queued < provider_resume < member_removal
note_03b = source.index("# NOTE_03B_VOLUNTARY_LEAVE", member_removal)
leave_target = source.index("note-03b-target-path-create-key-01", note_03b)
leave_invitation = source.index("note-03b-target-invitation-key-01", leave_target)
leave_acceptance = source.index("note-03b-target-accept-key-0001", leave_invitation)
leave_provider_pause = source.index("docker compose stop push-provider", leave_acceptance)
leave_nudge = source.index('"http://localhost:8080/v1/paths/$note_03b_target_path_id/members/$participant_id/nudges"', leave_provider_pause)
leave_retry_queued = source.index("note_03b_nudge_retry_queued", leave_nudge)
leave_provider_resume = source.index("docker compose start push-provider", leave_retry_queued)
voluntary_leave = source.index("note-03b-leave-key-000000001", leave_provider_resume)
assert note_03b < leave_target < leave_invitation < leave_acceptance < leave_provider_pause < leave_nudge < leave_retry_queued < leave_provider_resume < voluntary_leave
note_03c = source.index("# NOTE_03C_HIDDEN_RETAINED_ACTIVITY", voluntary_leave)
activity_membership = source.index("NOTE_03C_ACTIVITY_MEMBERSHIP", note_03c)
activity_authorization = source.index("TestSeedPathHTTPAcceptanceParticipant", activity_membership)
activity = source.index("note-03c-activity-create-key-0001", note_03c)
interaction_provider_pause = source.index("docker compose stop push-provider", activity)
comment = source.index("note-03c-comment-create-key-00001", interaction_provider_pause)
interaction_queued = source.index("note_03c_interaction_queued", comment)
interaction_leave = source.index("note-03c-leave-key-000000001", interaction_queued)
interaction_provider_resume = source.index("docker compose start push-provider", interaction_leave)
rejoin_membership = source.index("NOTE_03C_REJOIN_MEMBERSHIP", interaction_provider_resume)
rejoin_authorization = source.index("TestSeedPathHTTPAcceptanceParticipant", rejoin_membership)
assert note_03c < activity_membership < activity_authorization < activity < interaction_provider_pause < comment < interaction_queued < interaction_leave < interaction_provider_resume < rejoin_membership < rejoin_authorization
assert "(path_id, user_id, role, joined_at)" in source[activity_membership:activity_authorization]
assert "date_trunc('minute', created_at)" in source[activity_membership:activity_authorization]
assert "(path_id, user_id, role, joined_at)" in source[rejoin_membership:rejoin_authorization]
assert "'participant', CURRENT_TIMESTAMP" in source[rejoin_membership:rejoin_authorization]
note_03c_end = source.index("NOTE-03C retained-activity interaction cleanup", rejoin_authorization)
note_03c_proof = source[note_03c:note_03c_end]
assert "failure_code = 'path_access_revoked'" in note_03c_proof
assert "NOTE_03C_INTERACTION_QUEUED" in note_03c_proof
assert 'wait_for_push "$note_03c_manager_notification_id"' not in note_03c_proof
assert "token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL" in note_03c_proof
assert "delivered_at IS NULL AND permanently_failed_at IS NULL" in note_03c_proof
assert '"$note_03c_persistence" == \'1|1|1|1|0|1|1|1|1|1\'' in note_03c_proof
note_03d = source.index("# NOTE_03D_VISIBILITY_CONTRACTION_CLEANUP", note_03c_end)
interaction_path = source.index("note-03d-interaction-path-create-key-01", note_03d)
interaction_membership = source.index("NOTE_03D_ACTIVITY_MEMBERSHIP", interaction_path)
interaction_authorization = source.index("TestSeedPathHTTPAcceptanceParticipant", interaction_membership)
interaction_activity = source.index("NOTE_03D_ACTIVITY_SEED", interaction_authorization)
interaction_provider_pause = source.index("docker compose stop push-provider", interaction_activity)
interaction_comment = source.index("note-03d-comment-create-key-00001", interaction_provider_pause)
interaction_heart = source.index("note-03d-comment-heart-key-000001", interaction_comment)
interaction_queued = source.index("note_03d_interaction_queued", interaction_heart)
interaction_history = source.index("NOTE_03D_INTERACTION_HISTORY_READY", interaction_queued)
visibility_contraction = source.index("note-03d-visibility-private-key-01", interaction_history)
provider_resume = source.index("docker compose start push-provider", visibility_contraction)
visibility_expansion = source.index("note-03d-visibility-public-key-001", provider_resume)
note_03d_end = source.index("NOTE-03D visibility-contraction interaction cleanup", visibility_expansion)
assert note_03d < interaction_path < interaction_membership < interaction_authorization < interaction_activity < interaction_provider_pause < interaction_comment < interaction_heart < interaction_queued < interaction_history < visibility_contraction < provider_resume < visibility_expansion < note_03d_end
note_03d_proof = source[note_03d:note_03d_end]
assert "'participant', created_at" in source[interaction_membership:interaction_authorization]
assert "VALUES (:'participant_id', :'administrator_id', CURRENT_TIMESTAMP)" in source[interaction_membership:interaction_authorization]
assert '-acceptance-path-participant "$administrator_id"' in source[interaction_membership:interaction_activity]
assert "INSERT INTO recorded_activity_models" in source[interaction_activity:interaction_provider_pause]
assert "'note-03d-visibility-activity'" in source[interaction_authorization:interaction_provider_pause]
history_poll = source[interaction_queued:visibility_contraction]
assert "for _ in $(seq 1 70)" in history_poll
assert "note_03d_unread_before" not in note_03d_proof
assert "EXPECTED_UNREAD_COUNT" not in note_03d_proof
assert 'type(d["meta"]["unreadCount"]) is int' in note_03d_proof
assert 'item["read"] is False' in note_03d_proof
assert "failure_code = 'path_access_revoked'" in note_03d_proof
assert "token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL" in note_03d_proof
assert "delivered_at IS NULL AND permanently_failed_at IS NULL" in note_03d_proof
assert 'item["type"]=="comment_heart"' in note_03d_proof
visibility_proof = source.index("node scripts/web-path-visibility-acceptance.mjs")
follow_cleanup = source.index("path-05c-unfollow-key-00001")
follow_endpoint = source.index("http://localhost:8080/v1/profiles/live.acceptance.owner/follow", follow_cleanup)
name_only_path = source.index("live_path_create_key='live-path-create-key-00000001'")
assert visibility_proof < follow_cleanup < follow_endpoint < name_only_path
assert '-X DELETE' in source[visibility_proof:follow_cleanup]
PY
grep -Fq "getByRole('heading', { name: 'Path visibility', exact: true })" "$path_visibility"
grep -Fq "getByRole('alertdialog')" "$path_visibility"
grep -Fq "name: 'Confirm visibility change', exact: true" "$path_visibility"
grep -Fq "participant was shown creator-only visibility management" "$path_visibility"
grep -Fq "Current visibility: Private" "$path_visibility"
grep -Fq $'WEB_ACCEPTANCE_BASE_URL="$web_base_url" \\\n  WEB_ACCEPTANCE_API_URL="$api_base_url" \\\n  WEB_ACCEPTANCE_APPLICATION_TOKEN="$stranger_token"' "$live"
grep -Fq "import { chromium } from 'playwright'" "$subject"
grep -Fq "locale: 'es-ES'" "$subject"
grep -Fq "getAttribute('lang')" "$subject"
grep -Fq "accept-language" "$subject"
grep -Fq "Tu aplicación generada está lista." "$subject"
grep -Fq "Iniciar sesión" "$subject"
grep -Fq "const dexOrigin = process.env.WEB_ACCEPTANCE_DEX_ORIGIN ?? 'http://localhost:5556'" "$subject"
grep -Fq "input[name=login]" "$subject"
grep -Fq "input[name=password]" "$subject"
grep -Fq "Configura tu perfil" "$subject"
grep -Fq 'response.url() === `${apiBaseURL}/v1/onboarding`' "$subject"
grep -Fq "name: 'Correo del proveedor'" "$subject"
grep -Fq 'await privateEmail.inputValue()' "$subject"
grep -Fq "getByRole('textbox', { name: 'Nombre de usuario', exact: true })" "$subject"
grep -Fq "username.inputValue() !== 'developer'" "$subject"
grep -Fq "username.fill('reviewed.user')" "$subject"
grep -Fq "username.inputValue() !== 'reviewed.user'" "$subject"
grep -Fq 'const invitationID = process.env.WEB_ACCEPTANCE_INVITATION_ID' "$subject"
grep -Fq 'const invitationPathName = process.env.WEB_ACCEPTANCE_INVITATION_PATH_NAME' "$subject"
grep -Fq 'response.request().method() === '\''POST'\''' "$subject"
grep -Fq 'acceptanceRequests.length !== 0' "$subject"
grep -Fq "getByRole('alertdialog')" "$subject"
grep -Fq "name: 'Confirmar aceptación', exact: true" "$subject"
grep -Fq 'await expectFocused(confirmAcceptance' "$subject"
grep -Fq "name: 'Cancelar', exact: true" "$subject"
grep -Fq 'await expectFocused(acceptInvitation' "$subject"
grep -Fq 'invitation disappeared after cancelling its visibility warning' "$subject"
grep -Fq 'visibilityWarningAcknowledgement?.pathVisibility !== '\''followers'\''' "$subject"
grep -Fq 'accepted invitation remained on Home' "$subject"
grep -Fq 'getByRole('\''button'\'', { name: invitationPathName, exact: true })' "$subject"
grep -Fq 'browser Dex invitation visibility warning, cancel, confirmation, and Path projection acceptance passed' "$subject"
grep -Fq "onboarding?.data?.usernameSuggestion !== 'developer'" scripts/scalar-browser-acceptance.mjs
if grep -Fq 'response.url() === `${apiBaseURL}/v1/me`' "$subject"; then
  echo "first-sign-in browser acceptance must not probe the active profile route" >&2
  exit 1
fi
grep -Fq 'node scripts/scalar-browser-acceptance.mjs' "$live"
grep -Fq 'node scripts/web-browser-acceptance.mjs' "$live"
grep -Fq 'WEB_ACCEPTANCE_API_URL="$api_base_url" WEB_ACCEPTANCE_APPLICATION_TOKEN="$owner_token" node scripts/web-empty-home-acceptance.mjs' "$live"
grep -Fq 'provisional_owner_token="$(session_from_identity "$owner_identity_token" provisional-owner)"' "$live"
grep -Fq 'owner_active_identity_token="$(token hourpaths-web developer@example.com)"' "$live"
grep -Fq '"$owner_active_identity_token" != "$owner_identity_token"' "$live"
grep -Fq 'owner_token="$(session_from_identity "$owner_active_identity_token" owner)"' "$live"
grep -Fq "provisional-path-create-key-0001" "$live"
grep -Fq "live-path-create-key-00000001" "$live"
grep -Fq "live-timer-start-key-000001" "$live"
grep -Fq "live-timer-stop-key-0000001" "$live"
grep -Fq "live-simultaneous-start-key-001" "$live"
grep -Fq "live-simultaneous-stop-key-0001" "$live"
grep -Fq "sleep 2" "$live"
grep -Fq "recorded_activity_models" "$live"
grep -Fq "activity_mutation_models" "$live"
grep -Fq "authorization_outbox_models" "$live"
grep -Fq 'supporter_denied_timer="$(curl -sS -X POST -w '\''|%{http_code}'\''' "$live"
grep -Fq 'stranger_denied_timer="$(curl -sS -X POST -w '\''|%{http_code}'\''' "$live"
grep -Fq '"$supporter_denied_timer" == "$supporter_missing_timer"' "$live"
grep -Fq '"$stranger_denied_timer" == "$stranger_missing_timer"' "$live"
grep -Fq "<<'LIVE_ACTIVITY_SQL' | grep -Fx '1|1|4|1|1|1|1|1'" "$live"
grep -Fq 'reloaded timer state did not retain accumulated time' "$live"
grep -Fq 'stopping one Path altered its simultaneous timer' "$live"
grep -Fq 'first_entry.started_at < second_entry.ended_at' "$live"
grep -Fq "<<'LIVE_SIMULTANEOUS_ACTIVITY_SQL' | grep -Fx '2|2|1|1|1|1|1'" "$live"
grep -Fq 'two-Path timers retained two fully counted overlapping activities' "$live"
grep -Fq '/activities/manual-defaults' "$live"
grep -Fq 'live-manual-create-key-0001' "$live"
grep -Fq 'live-manual-update-key-0001' "$live"
grep -Fq 'recorded_activity_revision_models' "$live"
grep -Fq "operation = 'activity.manual.create'" "$live"
grep -Fq "operation = 'activity.update'" "$live"
grep -Fq 'live manual activity create, owner edit/replay, private note projection, Path history, and immutable revision acceptance passed' "$live"
grep -Fq 'live-manual-delete-key-0001' "$live"
grep -Fq 'manual_delete_response="$(curl -fsS -X DELETE' "$live"
grep -Fq 'expect_status 409 -X DELETE' "$live"
grep -Fq 'manual activity deletion did not replay the original result' "$live"
grep -Fq 'cross-owner and missing activity deletion responses differed' "$live"
grep -Fq 'result_activity_deleted' "$live"
grep -Fq "recorded_activity_revision_models WHERE activity_id = :'activity_id'" "$live"
grep -Fq "action = 'resource.access_denied' AND owner_user_id = :'participant_id' AND actor_user_id = :'participant_id' AND target_type = 'activity' AND target_id = :'activity_id' AND outcome = 'denied'" "$live"
grep -Fq "action = 'resource.access_denied' AND owner_user_id = :'participant_id' AND actor_user_id = :'participant_id' AND target_type = 'activity' AND target_id = :'missing_activity_id' AND outcome = 'denied'" "$live"
grep -Fq 'expected=0|0|1|1|4|3|1|1|1|0 actual=' "$live"
grep -Fq 'manual activity delete/replay, opaque ownership, cascade, tombstone, and unrelated preservation acceptance passed' "$live"
grep -Fq 'docker compose run --rm app-migrate -target-version 30' "$live"
grep -Fq 'expected clean current migration ledger 30|f' "$live"
grep -Fq 'activity deletion schema or least-privilege grants are incomplete' "$live"
grep -Fq '"$ledger" == '\''30|f'\''' "$live"
grep -Fq 'path_view_audit_before="$(successful_path_view_audit_count)"' "$live"
grep -Fq 'path_view_audit_after - path_view_audit_before == 4' "$live"
grep -Fq 'denied_path_audit_after - denied_path_audit_before == 1' "$live"
grep -Fq 'owner user.viewed audit count was not positive:' "$live"
grep -Fq 'owner Path-list audit count was below two:' "$live"
grep -Fq 'authorization recovery persistence mismatch: expected=1|1|1|1 actual=' "$live"
if grep -Fq "grep -Fx '4|1'" "$live"; then
  echo "live acceptance must scope Path audit assertions to the requests under test" >&2
  exit 1
fi
grep -Fq 'WEB_ACCEPTANCE_APPLICATION_TOKEN="$owner_token" WEB_ACCEPTANCE_PATH_NAME=' "$live"
grep -Fq "browser-progress-path-create-key-0001" "$live"
grep -Fq '"overallTarget":{"targetSeconds":120}' "$live"
grep -Fq '"intervalGoal":{"targetSeconds":60,"recurrence":"daily","alignment":{"hour":0}}' "$live"
grep -Fq "createManualActivity(pathID" apps/web/src/routes/+page.svelte
grep -Fq "updateActivity(pathID" apps/web/src/routes/+page.svelte
grep -Fq "createManualActivity(pathID" apps/mobile/app/index.tsx
grep -Fq "updateActivity(pathID" apps/mobile/app/index.tsx
grep -Fq "Home exposed manual activity entry" "$manual_activity"
grep -Fq 'locator(`[data-activity-id="${activityID}"]`)' "$manual_activity"
grep -Fq "createdBody?.data?.activity?.note !== 'browser private note'" "$manual_activity"
grep -Fq "detailBody?.data?.activity?.id !== activityID" "$manual_activity"
grep -Fq "browser unrelated note" "$manual_activity"
grep -Fq "readFileSync(new URL('../packages/i18n/src/locales/en.json', import.meta.url), 'utf8')" "$manual_activity"
grep -Fq "englishCatalog['pathDetails.deleteConfirmation']" "$manual_activity"
grep -Fq "cancelled deletion changed the authoritative total" "$manual_activity"
grep -Fq "response.request().method() === 'DELETE'" "$manual_activity"
grep -Fq "deletedResponse.request().headers()['idempotency-key']" "$manual_activity"
grep -Fq "browser deletion did not return the authoritative total" "$manual_activity"
grep -Fq "getByRole('progressbar'" "$manual_activity"
grep -Fq "0 of 120 seconds toward overall target" "$manual_activity"
grep -Fq "60 of 120 seconds toward overall target" "$manual_activity"
grep -Fq "0 of 60 seconds this interval" "$manual_activity"
grep -Fq "browser manual create did not return authoritative current interval progress" "$manual_activity"
grep -Fq "60 of 60 seconds — interval goal completed" "$manual_activity"
grep -Fq "editStartInstant = new Date(Date.parse(createdBody.data.activity.startedAt) - 120_000)" "$manual_activity"
grep -Fq "getByLabel('Start time', { exact: true }).fill(editStart.localTime)" "$manual_activity"
grep -Fq "133 of 120 seconds — overall target completed" "$manual_activity"
grep -Fq "150 of 120 seconds — overall target completed" "$manual_activity"
grep -Fq "17 of 120 seconds toward overall target" "$manual_activity"
grep -Fq "no-goal Path rendered overall progress" "$manual_activity"
grep -Fq "deleted activity survived the browser reload" "$manual_activity"
grep -Fq "browser Path manual create, reopen, edit, revision, cancel-delete, delete, and reload acceptance passed" "$manual_activity"
grep -Fq "WEB_ACCEPTANCE_PARTICIPANT_TOKEN is required" "$goal_update"
grep -Fq "WEB_ACCEPTANCE_PATH_ID is required" "$goal_update"
grep -Fq "45 of 60 seconds this interval" "$goal_update"
grep -Fq "45 of 30 seconds — interval goal completed" "$goal_update"
grep -Fq "locator('#manage-interval-hours').fill('0')" "$goal_update"
grep -Fq "locator('#manage-interval-minutes').fill('0')" "$goal_update"
grep -Fq "cancelled goal review issued a PUT" "$goal_update"
grep -Fq "confirmed: true" "$goal_update"
grep -Fq "headers()['idempotency-key']" "$goal_update"
grep -Fq "participant was shown Manage Path despite manageGoals=false" "$goal_update"
grep -Fq "participant capability-gated view issued a goal PUT" "$goal_update"
grep -Fq "participant view changed the owner goal projection" "$goal_update"
grep -Fq "goal removal retained interval progress" "$goal_update"
grep -Fq "45 seconds accumulated" "$goal_update"
grep -Fq 'WEB_ACCEPTANCE_PARTICIPANT_TOKEN="$participant_token"' "$live"
grep -Fq 'WEB_ACCEPTANCE_APPLICATION_TOKEN="$administrator_token"' "$live"
grep -Fq 'WEB_ACCEPTANCE_PATH_ID="$browser_goal_path_id"' "$live"
grep -Fq 'node scripts/web-goal-update-acceptance.mjs' "$live"
grep -Fq "browser-goal-path-create-key-0001" "$live"
grep -Fq "browser-goal-activity-create-key-001" "$live"
grep -Fq "BROWSER_GOAL_ADMINISTRATOR_ACTIVITY" "$live"
grep -Fq "BROWSER_GOAL_ROLE_MEMBERSHIPS" "$live"
grep -Fq "COMPLETE_ACTIVE_PATH_ROLE_FIXTURES" "$live"
grep -Fq 'acceptance-path-administrator "$administrator_id"' "$live"
grep -Fq "BROWSER_GOAL_UPDATE_EVIDENCE" "$live"
grep -Fq "principal_id = :'administrator_id' AND operation = 'path.goals.update'" "$live"
grep -Fq "operation = 'path.goals.update'" "$live"
grep -Fq "browser goal update confirmation, capability gating, removal, and persistence acceptance passed" "$live"
grep -Fq "PATH_07_ARCHIVE_ACCEPTANCE" "$live"
grep -Fq 'docker compose run --rm app-migrate -target-version 31' "$live"
grep -Fq 'MIGRATION_PATH_ARCHIVE_SCHEMA' "$live"
grep -Fq '000031_path_archival.down.sql' "$live"
grep -Fq "expected clean current migration ledger 31|f" "$live"
grep -Fq "path-07-confirmed-archive-01" "$live"
grep -Fq '{"confirmed":false,"expectedArchived":false,"archived":true}' "$live"
grep -Fq '{"confirmed":true,"expectedArchived":false,"archived":true}' "$live"
grep -Fq '{"confirmed":true,"expectedArchived":true,"archived":false}' "$live"
grep -Fq "'http://localhost:8080/v1/paths?archived=true&limit=100'" "$live"
grep -Fq "PATH_07_EVIDENCE" "$live"
grep -Fq "path.archive-state.set" "$live"
grep -Fq "PATH-07 creator confirmation, timer save, archived projection/read-only state, and non-restarting unarchive acceptance passed" "$live"
grep -Fq "PATH_03_INVITATION_ACCEPTANCE" "$live"
grep -Fq '/v1/paths/$path_03_path_id/invitation-recipient?username=live.acceptance.stranger' "$live"
grep -Fq 'path_03_send_body=' "$live"
grep -Fq '"offeredRole":"participant"' "$live"
grep -Fq '"$path_03_replayed_send" == "$path_03_send_response"' "$live"
grep -Fq '/v1/path-invitations?limit=25' "$live"
test "$(grep -Fc '/v1/notifications?limit=25' "$live")" -eq 3
grep -Fq 'item["type"]=="path_invitation_received" and item["presentation"]=="actionable"' "$live"
grep -Fq 'item["type"]=="path_invitation_accepted" and item["presentation"]=="informational"' "$live"
grep -Fq 'item["pathName"]=="Invitation acceptance" and item["inviter"]==' "$live"
grep -Fq '/v1/path-invitations/$path_03_invitation_id/accept' "$live"
grep -Fq '"$path_03_replayed_accept" == "$path_03_accept_response"' "$live"
grep -Fq 'PATH_03_PERSISTENCE_EVIDENCE' "$live"
grep -Fq 'SOC03B_PATH_CHRONOLOGY_FIXTURE' "$live"
grep -Fq "SET joined_at = (SELECT created_at FROM path_models WHERE id = :'path_id')" "$live"
grep -Fq '[[ "$soc03b_chronology_count" == 1 ]]' "$live"
grep -Fq 'LIVE_MANUAL_ACTIVITY_CHRONOLOGY_FIXTURE' "$live"
grep -Fq '[[ "$live_path_chronology_count" == 1 ]]' "$live"
grep -Fq 'BROWSER_MANUAL_ACTIVITY_CHRONOLOGY_FIXTURE' "$live"
grep -Fq '[[ "$browser_progress_chronology_count" == 1 ]]' "$live"
grep -Fq 'BROWSER_GOAL_ACTIVITY_CHRONOLOGY_FIXTURE' "$live"
grep -Fq '[[ "$browser_goal_chronology_count" == 1 ]]' "$live"
grep -Fq "operation = 'path.invitation.send'" "$live"
grep -Fq "operation = 'path.invitation.accept'" "$live"
grep -Fq "kind = 'path_invitation_accepted'" "$live"
grep -Fq '"capabilities":{"trackTime":True,"inviteMembers":False,"manageMembers":False,"manageGoals":False,"manageLifecycle":False,"manageVisibility":False,"renamePath":False,"transferOwnership":False,"leavePath":True}' "$live"
if grep -E '"capabilities".*"renamePath"' "$live" | grep -Fv '["renamePath"]' | grep -Fv '"manageMembers"'; then
  echo "live acceptance contains an exact Path capability projection without member management" >&2
  exit 1
fi
if grep -E '"capabilities".*"renamePath"' "$live" | grep -Fv '["renamePath"]' | grep -Fv '"transferOwnership"'; then
  echo "live acceptance contains an exact Path capability projection without ownership transfer" >&2
  exit 1
fi
if grep -E '"capabilities".*"renamePath"' "$live" | grep -Fv '["renamePath"]' | grep -Fv '"leavePath"'; then
  echo "live acceptance contains an exact Path capability projection without leave capability" >&2
  exit 1
fi
grep -Fq 'COMPOSE_FILE="$root/compose.yaml:$root/compose.acceptance.yaml"' "$live"
grep -Fq 'HOURPATHS_PUSH_PROVIDER_ENDPOINT="http://push-provider:19090"' "$live"
grep -Fq 'HOURPATHS_PUSH_PROVIDER_LOG_PATH="$push_provider_log"' "$live"
grep -Fq 'scripts/fake-expo-push-provider.mjs' "$root/compose.acceptance.yaml"
grep -Fq 'profiles: [acceptance]' "$root/compose.acceptance.yaml"
if grep -Fq 'host.docker.internal' "$live"; then
  echo "live push acceptance must not depend on Docker host-gateway routing" >&2
  exit 1
fi
if grep -Fq 'host.docker.internal' "$root/compose.yaml"; then
  echo "Compose must not retain an unused Docker host-gateway mapping" >&2
  exit 1
fi
grep -Fq '/v1/push-installations/path-03-stranger-installation' "$live"
grep -Fq '/v1/push-installations/path-03-owner-installation' "$live"
grep -Fq 'wait_for_push "$path_03_received_notification_id"' "$live"
grep -Fq 'wait_for_push "$path_03_accepted_notification_id"' "$live"
grep -Fq 'FROM notification_push_delivery_models' "$live"
grep -Fq "PATH-03 invitation send, localized push delivery, user-visible notices, recipient Home projection, exact participant capability, and replay persistence acceptance passed" "$live"
grep -Fq "PATH_04_PRIVATE_PROFILE_WARNING" "$live"
grep -Fq 'path_04_warning_context_before_change' "$live"
grep -Fq 'path_04_warning_context_after_change' "$live"
grep -Fq '["code"]=="invitation_warning_required"' "$live"
grep -Fq 'PATH_04_FAILED_ACCEPTANCE_EVIDENCE' "$live"
grep -Fq "operation = 'path.invitation.accept'" "$live"
grep -Fq "kind = 'path_invitation_accepted'" "$live"
grep -Fq 'PATH_04_CONTROL_INVITATIONS' "$live"
grep -Fq 'assert "warning" not in matches[os.environ["SUPPORTER_INVITATION_ID"]]' "$live"
grep -Fq 'assert "warning" not in matches[os.environ["PRIVATE_PATH_INVITATION_ID"]]' "$live"
grep -Fq 'assert "warning" not in matches[os.environ["PUBLIC_PROFILE_INVITATION_ID"]]' "$live"
grep -Fq 'PATH_04_RETAINED_ACTIVITY' "$live"
grep -Fq '"hasRetainedActivity":True' "$live"
grep -Fq 'path-04-retained-activity' "$live"
grep -Fq 'PATH_04_ACCEPTED_EVIDENCE' "$live"
grep -Fq "PATH-04 current-visibility warning, rollback, controls, atomic acceptance, and retained-activity rejoin acceptance passed" "$live"
grep -Fq 'd["accumulatedSeconds"]==50 and d["intervalProgress"]=={"accumulatedSeconds":50,"targetSeconds":60}' "$live"
if grep -Fq 'json.load(sys.stdin)["data"]=={"accumulatedSeconds":50}' "$live"; then
  echo "goal-achievement deletion acceptance must tolerate the documented intervalProgress projection" >&2
  exit 1
fi
grep -Fq '"sessionCount":1,"unreadNotificationCount":d["unreadNotificationCount"],"removedFeedEventIds":["practice:"+os.environ["MANUAL_ACTIVITY_ID"]]' "$live"
grep -Fq 'd["meta"]["unreadCount"]==int(os.environ["MANUAL_DELETE_UNREAD_COUNT"])' "$live"
grep -Fq "result_unread_notification_count = :'delete_unread_count'::bigint" "$live"
grep -Fq "result_removed_feed_event_ids = ARRAY['practice:' || :'activity_id']::text[]" "$live"
grep -Fq 'SOC-04D_ACHIEVEMENT_INTERACTIONS' "$live"
grep -Fq 'soc04d-achievement-reaction-key-01' "$live"
grep -Fq 'soc04d-achievement-comment-key-001' "$live"
grep -Fq 'soc04d-owner-heart-key-000001' "$live"
grep -Fq 'soc04d-author-heart-key-0001' "$live"
grep -Fq 'soc04d_comment_notification_id' "$live"
grep -Fq 'wait_for_push "$soc04d_comment_notification_id"' "$live"
grep -Fq "kind = 'practice_comment'" "$live"
grep -Fq "notification.comment_id = :'comment_id'" "$live"
grep -Fq 'comments/$soc04d_comment_id/hearts?limit=1' "$live"
grep -Fq 'SOC04D_ROSTER_CURSOR' "$live"
grep -Fq 'social_practice_comment_heart_models' "$live"
grep -Fq 'SOC-04D achievement reaction, comment, paged heart roster, and invalidation cleanup acceptance passed' "$live"
grep -Fq 'SOC-05_PROFILE_INTERACTION_CONTROLS' "$live"
grep -Fq '/v1/me/interaction-settings' "$live"
grep -Fq 'soc05-disable-comments-key-01' "$live"
grep -Fq 'soc05-enable-comments-key-001' "$live"
grep -Fq 'soc05-disable-reactions-key-1' "$live"
grep -Fq 'soc05-enable-reactions-key-01' "$live"
grep -Fq 'SOC05_HIDDEN_STORAGE_EVIDENCE' "$live"
grep -Fq 'SOC05_EXACT_EVENT_RESOLUTION' "$live"
grep -Fq 'social_practice_comment_models' "$live"
grep -Fq 'social_practice_reaction_models' "$live"
grep -Fq 'notification_push_delivery_models' "$live"
grep -Fq 'SOC-05 independent comment/reaction disable, cleanup, denial, and restore acceptance passed' "$live"
if grep -Fq 'soc03b-$soc03b_kind-reaction-key-01' "$live" || grep -Fq 'Not available on achievements' "$live"; then
  echo "SOC-04D acceptance must exercise positive achievement engagement instead of legacy opaque denials" >&2
  exit 1
fi
if grep -Fq 'reacted to your practice' "$root/apps/api/internal/app/push/worker.go" ||
  grep -Fq 'reaccionó a tu práctica' "$root/apps/api/internal/app/push/worker.go" ||
  grep -Fq 'to your practice' "$root/packages/i18n/src/locales/en.json" ||
  grep -Fq 'a tu práctica' "$root/packages/i18n/src/locales/es.json"; then
  echo "feed-event notification copy must not describe achievement engagement as practice" >&2
  exit 1
fi
grep -Fq "SOC-03B mixed goal-achievement feed lifecycle acceptance passed" "$live"
grep -Fq "PATH_04_BROWSER_WARNING" "$live"
grep -Fq 'WEB_ACCEPTANCE_INVITATION_ID="$path_04_browser_invitation_id"' "$live"
grep -Fq 'WEB_ACCEPTANCE_INVITATION_PATH_NAME='\''Browser visibility warning'\''' "$live"
grep -Fq 'PATH_04_BROWSER_ACCEPTANCE_EVIDENCE' "$live"
grep -Fq 'browser Dex visibility-warning acceptance persistence passed' "$live"
test "$(grep -Fc 'if item.get("invitationId")==os.environ["INVITATION_ID"]' "$live")" -eq 2
test "$(grep -Fc 'type(d["meta"]["unreadCount"]) is int and d["meta"]["unreadCount"]>=1 and len(items)==1' "$live")" -eq 2
printf '%s' '{"meta":{"unreadCount":2},"data":[{"type":"path_visibility_changed"},{"invitationId":"expected"}]}' |
  INVITATION_ID=expected python3 -c 'import json,os,sys;d=json.load(sys.stdin);items=[item for item in d["data"] if item.get("invitationId")==os.environ["INVITATION_ID"]];assert type(d["meta"]["unreadCount"]) is int and d["meta"]["unreadCount"]>=1 and items==[{"invitationId":"expected"}]'
test "$(grep -Fc "PATH_05C_ACTIVITY_SQL" "$live")" -eq 4
test "$(grep -Fc "PATH_05C_PROFILE_SQL" "$live")" -eq 4
if sed -n '/# PATH-05C proves/,/PATH-05C creator-only expansion\/contraction/p' "$live" | grep -Eq -- "-c .*:'"; then
  echo "PATH-05C psql variables must always be evaluated from stdin rather than -c" >&2
  exit 1
fi
grep -Fq "new active account did not have an empty server-backed Path collection" "$empty_home"
grep -Fq "getByRole('button', { name: 'Crear ruta', exact: true }).click()" "$empty_home"
grep -Fq 'docker compose exec -T postgres psql -Atq -U app_migrator' "$live"
grep -Fq '127.0.0.1:${HOURPATHS_POSTGRES_HOST_PORT:-5432}:5432' "$root/compose.yaml"
grep -Fq '127.0.0.1:${HOURPATHS_DEX_HOST_PORT:-5556}:5556' "$root/compose.yaml"
grep -Fq '127.0.0.1:${HOURPATHS_API_HOST_PORT:-8080}:8080' "$root/compose.yaml"
grep -Fq '127.0.0.1:${HOURPATHS_SPICEDB_HOST_PORT:-50051}:50051' "$root/compose.yaml"
grep -Fq '127.0.0.1:${HOURPATHS_WEB_HOST_PORT:-5173}:5173' "$root/compose.yaml"
grep -Fq 'HOURPATHS_CORS_ALLOWED_ORIGINS: ${HOURPATHS_WEB_PUBLIC_BASE_URL:-http://localhost:5173}' "$root/compose.yaml"
grep -Fq '${HOURPATHS_DEX_CONFIG_PATH:-./deploy/dex/config.yaml}:/etc/dex/config.yaml:ro' "$root/compose.yaml"
test "$(grep -Fc 'HOURPATHS_OIDC_ISSUER: ${HOURPATHS_DEX_PUBLIC_ISSUER:-http://localhost:5556/dex}' "$root/compose.yaml")" -eq 2
grep -Fq 'HOURPATHS_REQUESTS_PER_MINUTE: ${HOURPATHS_REQUESTS_PER_MINUTE:-300}' "$root/compose.yaml"
grep -Fq 'HOURPATHS_AUDIT_EVENTS_PER_MINUTE: ${HOURPATHS_AUDIT_EVENTS_PER_MINUTE:-120}' "$root/compose.yaml"
grep -Fq 'export HOURPATHS_REQUESTS_PER_MINUTE=10000' "$live"
grep -Fq 'export HOURPATHS_AUDIT_EVENTS_PER_MINUTE=10000' "$live"
grep -Fq 'unset HOURPATHS_REQUESTS_PER_MINUTE' "$live"
grep -Fq 'unset HOURPATHS_AUDIT_EVENTS_PER_MINUTE' "$live"
grep -Fq 'postgres_host_port="${HOURPATHS_POSTGRES_HOST_PORT:-25432}"' "$live"
grep -Fq 'export HOURPATHS_POSTGRES_HOST_PORT="$postgres_host_port"' "$live"
grep -Fq 'lock_dir="/tmp/hourpaths-live-acceptance.lock"' "$live"
[[ "$(grep -Fc 'json.load(sys.stdin)["code"]=="invitation_warning_required"' "$live")" -eq 2 ]]
grep -Fq 'dex_host_port="${HOURPATHS_OIDC_HOST_PORT:-${HOURPATHS_DEX_HOST_PORT:-25556}}"' "$live"
grep -Fq 'api_host_port="${HOURPATHS_API_HOST_PORT:-28080}"' "$live"
grep -Fq 'spicedb_host_port="${HOURPATHS_SPICEDB_HOST_PORT:-25051}"' "$live"
grep -Fq 'web_host_port="${HOURPATHS_WEB_HOST_PORT:-25173}"' "$live"
grep -Fq 'api_base_url="http://localhost:${api_host_port}"' "$live"
grep -Fq 'web_base_url="${HOURPATHS_WEB_PUBLIC_BASE_URL:-http://localhost:${web_host_port}}"' "$live"
grep -Fq 'command curl "${rewritten[@]}"' "$live"
grep -Fq 'dex_public_issuer="http://localhost:${dex_host_port}/dex"' "$live"
grep -Fq 'dex_config_path="$(mktemp /tmp/hourpaths-dex-config.XXXXXX.yaml)"' "$live"
grep -Fq 'sed "s|^issuer: .*$|issuer: ${dex_public_issuer}|" deploy/dex/config.yaml >"$dex_config_path"' "$live"
grep -Fq 'chmod 0644 "$dex_config_path"' "$live"
grep -Fq 'rm -f "$dex_config_path"' "$live"
grep -Fq 'curl -fsS "${dex_public_issuer}/.well-known/openid-configuration"' "$live"
grep -Fq 'curl -fsS -X POST "${dex_public_issuer}/token"' "$live"
grep -Fq 'python3 scripts/identity-fixture.py user-id "$dex_public_issuer" "$1"' "$live"
grep -Fq -- '-v "identity_issuer=$dex_public_issuer"' "$live"
grep -Fq "(:'identity_issuer'" "$live"
grep -Fq 'WEB_ACCEPTANCE_DEX_ORIGIN="${dex_public_issuer%/dex}"' "$live"
grep -Fq 'SCALAR_ACCEPTANCE_BASE_URL="$api_base_url"' "$live"
grep -Fq 'WEB_ACCEPTANCE_API_URL="$api_base_url"' "$live"
test "$(grep -Fc 'WEB_ACCEPTANCE_BASE_URL="$web_base_url"' "$live")" -eq 7
grep -Fq 'sed -i.bak "s|http://localhost:5173|${web_base_url}|g" "$dex_config_path"' "$live"
grep -Fq 'export HOURPATHS_TERMS_URL="$web_base_url/legal/terms"' "$live"
grep -Fq 'export HOURPATHS_PRIVACY_POLICY_URL="$web_base_url/legal/privacy"' "$live"
grep -Fq 'export HOURPATHS_COMMUNITY_GUIDELINES_URL="$web_base_url/community-guidelines"' "$live"
grep -Fq 'export HOURPATHS_SUPPORT_URL="$web_base_url/support"' "$live"
grep -Fq '[[ ! "$postgres_host_port" =~ ^[1-9][0-9]{0,4}$ ]]' "$live"
grep -Fq '10#$postgres_host_port > 65535' "$live"
test "$(grep -c '@127.0.0.1:${postgres_host_port}/app' "$live")" -eq 3
browser_line="$(grep -nF 'node scripts/web-browser-acceptance.mjs' "$live" | cut -d: -f1 | head -n 1)"
fixture_line="$(grep -nF 'COMPLETE_ACTIVE_OWNER_FIXTURE' "$live" | cut -d: -f1 | head -n 1)"
owner_line="$(grep -nF 'owner_token="$(session_from_identity "$owner_active_identity_token" owner)' "$live" | cut -d: -f1 | head -n 1)"
(( browser_line < fixture_line && fixture_line < owner_line )) || {
	echo "complete active-owner fixture must follow provisional onboarding acceptance and precede active API checks" >&2
	exit 1
}
for activation_table in user_account_activation_models user_policy_acceptance_models user_preference_models user_time_zone_history_models; do
	grep -Fq "INSERT INTO $activation_table" "$live"
done
grep -Fq "username = 'live.acceptance.owner', profile_visibility = 'private'" "$live"
grep -Fq "scopes = 'api:onboarding' AND revoked_at IS NULL" "$live"
grep -Fq 'web_image="$(docker compose images -q web)"' "$live"
grep -Fq 'assert_web_image_rejects_config missing' "$live"
grep -Fq 'assert_web_image_rejects_config malformed-environment' "$live"
grep -Fq 'assert_web_image_rejects_config malformed-api-url' "$live"
grep -Fq 'assert_web_image_rejects_config local-api-url' "$live"
grep -Fq 'assert_web_image_rejects_config credentialed-issuer' "$live"
grep -Fq 'assert_web_image_rejects_config non-https-issuer' "$live"
grep -Fq 'assert_web_image_rejects_config blank-client-id' "$live"
grep -Fq 'docker run --detach --name "$container_name"' "$live"
grep -Fq 'for _ in $(seq 1 15)' "$live"
grep -Fq 'docker rm --force "$container_name"' "$live"
grep -Fq './scripts/test-web-browser-acceptance.sh' "$makefile"
grep -Fq '"scripts/web-browser-acceptance.mjs"' "$classifier"
grep -Fq '"scripts/web-browser-navigation.mjs"' "$classifier"
grep -Fq '"scripts/web-browser-navigation.test.mjs"' "$classifier"
grep -Fq '"scripts/web-empty-home-acceptance.mjs"' "$classifier"
grep -Fq '"scripts/web-goal-update-acceptance.mjs"' "$classifier"
grep -Fq '"scripts/test-web-browser-acceptance.sh"' "$classifier"

echo "web browser acceptance contract test passed"
