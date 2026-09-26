import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';
import { nudgeActionState } from './nudge-presentation';

function source(path: string): string {
  return readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
}

const app = source('../app/index.tsx');
const layout = source('../app/_layout.tsx') + source('./ui/navigation-theme.ts');
const memberView = source('./ui/path-member-management-view.tsx');
const composer = source('./ui/nudge-composer-sheet.tsx');
const audienceView = source('./ui/path-nudge-settings-view.tsx');
const audienceRoute = source('../app/path/[pathID]/nudge-settings.tsx');
const pathRecovery = source('./path-route-recovery.ts');
const pathAncestry = source('./path-route-ancestry.ts');
const notificationSettings = source('../app/settings/notifications.tsx');
const settingsPresentation = source('./ui/settings-presentation.tsx');

test('send action appears only for an authoritatively eligible tracking participant', () => {
  const base = { isViewer: false, role: 'participant' as const };
  assert.deepEqual(nudgeActionState({
    ...base,
    nudgeEligibility: { eligible: true, pathId: 'path-1', recipientUserId: 'user-2' },
  }), { kind: 'send' });
  assert.deepEqual(nudgeActionState({
    ...base,
    nudgeEligibility: { eligible: false, pathId: 'path-1', reason: 'goal_complete', recipientUserId: 'user-2' },
  }), { kind: 'goal_complete' });
  assert.deepEqual(nudgeActionState({
    ...base,
    nudgeEligibility: { eligible: false, pathId: 'path-1', reason: 'rate_limited', recipientUserId: 'user-2' },
  }), { kind: 'rate_limited' });
  assert.deepEqual(nudgeActionState({ ...base, isViewer: true }), { kind: 'hidden' });
  assert.deepEqual(nudgeActionState({ ...base, role: 'supporter' }), { kind: 'hidden' });
  assert.deepEqual(nudgeActionState(base), { kind: 'hidden' });
});

test('participant detail loads authoritative eligibility and owns one retry-safe send operation', () => {
  assert.match(app, /getPathMemberNudgeEligibility\(member\.pathId, member\.userId\)/);
  assert.match(app, /nudgeEligibilityFromAPI\(envelope\.data\)/);
  assert.match(app, /createNudgeSendOperationOwner\(\(\) => Crypto\.randomUUID\(\)\)/);
  assert.match(app, /sendPathMemberNudge\(pathId, recipientUserId, body, idempotencyKey\)/);
  assert.match(app, /reviewNudgeSend\(member\.pathId, member\.userId, preset\)/);
  assert.match(app, /setNudgeComposerPreset\(null\)/);
  assert.match(app, /AccessibilityInfo\.announceForAccessibility\(i18n\.t\('nudge\.compose\.sent'\)\)/);
  assert.match(memberView, /nudgeActionState\(member\)/);
  assert.match(memberView, /nudge\.sendAction/);
  assert.match(memberView, /nudge\.eligibility\.goalComplete/);
  assert.match(memberView, /nudge\.eligibility\.rateLimited/);
});

test('native composer is a compact five-preset single-choice sheet with no free text', () => {
  assert.match(composer, /<NativeSheet/);
  assert.match(composer, /title=\{i18n\.t\('nudge\.compose\.title'/);
  assert.match(composer, /NUDGE_PRESETS\.map/);
  assert.match(composer, /accessibilityRole="radio"/);
  assert.match(composer, /accessibilityState=\{\{ disabled: busy, selected/);
  assert.match(composer, /SettingsIcon[\s\S]*checkmark/);
  assert.match(composer, /leadingAction=\{\{[\s\S]*nudge\.compose\.cancel/);
  assert.match(composer, /trailingAction=\{\{[\s\S]*nudge\.compose\.send/);
  assert.match(composer, /disabled: busy \|\| selectedPreset === null/);
  assert.match(composer, /AccessibilityInfo\.isReduceMotionEnabled/);
  assert.match(composer, /accessibilityRole="radiogroup"/);
  assert.match(composer, /accessibilityLiveRegion="polite"[\s\S]*nudge\.compose\.sending/);
  assert.match(composer, /const currentActionKey = errorText \? 'common\.retry' : 'nudge\.compose\.send'/);
  assert.match(composer, /setAdmittedActionKey\(currentActionKey\)[\s\S]*onSend\(\)/);
  assert.match(composer, /busy && admittedActionKey \? admittedActionKey : currentActionKey/);
  assert.doesNotMatch(composer, /label: i18n\.t\(busy \? 'nudge\.compose\.sending'/);
  assert.doesNotMatch(composer, /TextInput|customMessage|messageInput|placeholder/);
  assert.match(app, /selectedPreset=\{nudgeComposerPreset\}/);
  assert.match(app, /dismissible=\{![^}]*nudgeSendBusy\}/);
});

test('each tracking participant has a dedicated per-Path native audience screen', () => {
  assert.match(app, /getPathNudgePreference\(pathId\)/);
  assert.match(app, /updatePathNudgePreference\(pathId, body, idempotencyKey\)/);
  assert.match(app, /createNudgeAudienceOperationOwner\(\(\) => Crypto\.randomUUID\(\)\)/);
  assert.match(app, /nudge\.audience\.openLabel/);
  assert.match(app, /router\.push\(\{ pathname: '\/path\/\[pathID\]\/nudge-settings'/);
  assert.match(audienceRoute, /pathNudgeSettingsRouteKey\(pathID\)/);
  assert.match(audienceRoute, /usePathRouteAncestry\(\{ kind: 'nudge-settings'/);
  assert.match(audienceRoute, /NativeRouteRecoveryView/);
  assert.match(audienceRoute, /onGoHome=/);
  assert.match(audienceRoute, /beforeRemove/);
  assert.doesNotMatch(audienceRoute, /return null/);
  assert.match(audienceRoute, /NativeRouteScreen grouped/);
  assert.match(layout, /path\/\[pathID\]\/nudge-settings/);
  assert.match(audienceView, /NUDGE_AUDIENCES\.map/);
  assert.match(audienceView, /accessibilityRole="radio"/);
  assert.match(audienceView, /accessibilityRole="radiogroup"/);
  assert.match(audienceView, /if \(!selected\) onSelect\(audience\)/);
  assert.match(audienceView, /SettingsIcon[\s\S]*checkmark/);
  assert.match(audienceView, /nudge\.audience\.footer/);
  assert.match(audienceView, /accessibilityLiveRegion="polite"/);
  assert.match(audienceView, /recoveryState/);
  assert.match(audienceView, /NativeRouteRecoveryView/);
  assert.match(audienceView, /onGoHome/);
  assert.match(pathRecovery, /kind: 'nudge-settings'/);
  assert.match(pathAncestry, /path\/\[pathID\]\/nudge-settings/);
});

test('Notifications settings owns an independent revisioned Nudges channel switch', () => {
  assert.match(settingsPresentation, /getNudgeChannelPreference/);
  assert.match(settingsPresentation, /updateNudgeChannelPreference/);
  assert.match(app, /getNudgeNotificationChannel\(\)/);
  assert.match(app, /updateNudgeNotificationChannel\(body, idempotencyKey\)/);
  assert.match(notificationSettings, /SettingsSwitchRow/);
  assert.match(notificationSettings, /notification\.settings\.nudges/);
  assert.match(notificationSettings, /nudgeChannelPreference/);
  assert.match(notificationSettings, /accessibilityLiveRegion="polite"/);
  assert.match(notificationSettings, /nudge\.channel\.saveError/);
});

test('received nudge notifications use ordinary Path context and stale targets stay opaque', () => {
  const context = app.slice(
    app.indexOf('async function openNotificationContext'),
    app.indexOf('function openPathSharing'),
  );
  assert.match(context, /notification\.type === 'nudge_received'/);
  assert.match(context, /openPathDetail\(path\.id\)/);
  assert.doesNotMatch(context, /nudge-history|received-nudges|nudgeId/);
  assert.match(app, /notification\.type === 'nudge_received'[\s\S]*kind: 'path'/);
  assert.match(app, /router\.replace\('\/\(tabs\)\/home'\)/);
});
