import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const app = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

test('nudge audience load and mutation publish only through the owned session lineage', () => {
  assert.match(app, /createPathNudgePreferenceTarget\(ownerID, currentSession\.token, pathId\)/);
  assert.match(app, /rotatePathNudgePreferenceTarget\([\s\S]*credential\.token/);
  assert.match(app, /ownsPathNudgePreferenceTarget\([\s\S]*latest\.session\?\.token,[\s\S]*pathId/);
  assert.match(app, /pathNudgeFailureDisposition\([\s\S]*requestSessionToken: currentSession\.token/);
  assert.match(app, /pathNudgePreferenceAdmission\.current[\s\S]*Symbol\('path-nudge-preference'\)/);
  assert.match(app, /pathNudgePreferenceAdmission\.current !== admission\) return/);
  assert.doesNotMatch(app, /pathNudgePreferenceTarget\.current\s*=\s*\{\s*ownerID,\s*pathID:\s*pathId,\s*session:/);
});

test('nudge send admission is synchronous and only its owner may release busy presentation', () => {
  assert.match(app, /nudgeSendAdmission\.current \|\|[\s\S]*Symbol\('nudge-send'\)/);
  assert.match(app, /nudgeSendAdmission\.current !== admission\) return/);
  assert.match(app, /nudgeSendAdmission\.current === admission[\s\S]*nudgeSendAdmission\.current = null/);
  assert.match(app, /function closeNudgeComposer\(\)[\s\S]*nudgeSendAdmission\.current\) return/);
  assert.match(app, /function selectNudgeComposerPreset\(preset: NudgePreset\)[\s\S]*setNudgeComposerPreset\(preset\);[\s\S]*setNudgeSendErrorKey\(null\)/);
  assert.match(app, /onSelect=\{selectNudgeComposerPreset\}/);
});

test('cold nudge settings resolves through the authoritative Path route and never depends on warm memory', () => {
  assert.match(app, /intent\.kind === 'nudge-settings'[\s\S]*loadPathNudgePreference\(intent\.pathID, false\)/);
  assert.match(app, /currentPathRouteIntent\?\.kind === 'nudge-settings' && \(!selectedPath \|\| !pathNudgeSettingsOpen\)[\s\S]*PathNudgeSettingsView/);
  assert.match(app, /recoveryState=\{pathRouteRecovery \?\? 'loading'\}/);
  assert.match(app, /onGoHome=\{leavePathRouteToHome\}/);
});

test('closing or replacing the nudge journey invalidates load, mutation, and route ownership together', () => {
  const close = app.slice(app.indexOf('function closePathNudgeSettings'), app.indexOf('function closePathMemberRemovalRoute'));
  assert.match(close, /nudgeAudienceLoadOperations\.invalidate\(\)/);
  assert.match(close, /nudgeAudienceOperations\.cancel\(\)/);
  assert.match(close, /if \(!force && pathNudgePreferenceAdmission\.current\) return/);
  assert.match(close, /pathNudgePreferenceTarget\.current = null/);
  assert.match(close, /pathRouteTarget\.current = null/);
  assert.match(app, /else \{\s*closePathNudgeSettings\(true\);\s*\}/);
});
