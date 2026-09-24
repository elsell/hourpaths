import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import test from 'node:test';

const source = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('settings operations retain arbitrary same-owner credential lineage', () => {
  assert.match(source, /settingsOperationTargets = useRef\(new Map/);
  assert.match(source, /for \(const \[intentKey, target\] of settingsOperationTargets\.current\)[\s\S]*rotateNotificationSessionTarget\(target, ownerID, credential\)/);
  assert.match(source, /settingsOperationTargets\.current\.values\(\)[\s\S]*target\.ownerID !== nextOwnerID[\s\S]*resetSettingsOperations\(\)/);
});

test('time-zone and interaction operations use owned targets instead of exact Session identity', () => {
  for (const intent of [
    'settings:time-zone:load',
    'settings:time-zone:update:',
    'settings:interactions:load',
    'settings:interactions:update:',
  ]) assert.match(source, new RegExp(intent.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  assert.match(source, /ownsNotificationSessionTarget\([\s\S]*settingsOperationTargets\.current\.get\(intentKey\)/);
});

test('settings mutations admit synchronously and stale cleanup cannot release a newer owner', () => {
  assert.match(source, /settingsMutationAdmissions\.current\.has\('time-zone'\)[\s\S]*Symbol\('settings-time-zone'\)/);
  assert.match(source, /settingsMutationAdmissions\.current\.get\('time-zone'\) === admission[\s\S]*delete\('time-zone'\)/);
  assert.match(source, /settingsMutationAdmissions\.current\.has\('interactions'\)[\s\S]*Symbol\('settings-interactions'\)/);
  assert.match(source, /settingsMutationAdmissions\.current\.get\('interactions'\) === admission[\s\S]*delete\('interactions'\)/);
});

test('replacement and sign-out synchronously invalidate settings operations', () => {
  assert.match(source, /function resetSettingsOperations\(\)[\s\S]*settingsOperationTargets\.current\.clear\(\)[\s\S]*settingsMutationAdmissions\.current\.clear\(\)/);
  assert.match(source, /resetNotifications\(\);\s*resetSettingsOperations\(\);\s*resetSocialProfileDiscovery\(\)/);
});

test('cold Settings journeys resume only for the owning session generation', () => {
  assert.match(source, /takeSettingsJourneyBootstrap\(socialPresentationKey\)/);
  assert.match(source, /router\.push\(settingsJourneyHref\(pending\)/);
  assert.match(source, /<SettingsJourneyRecoverySource[\s\S]*sessionKey=\{socialPresentationKey\}/);
  assert.match(source, /<UserBlockingRouteSource[\s\S]*sessionKey=\{socialPresentationKey\}/);
});
