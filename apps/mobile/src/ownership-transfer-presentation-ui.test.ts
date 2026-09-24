import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const sheet = readFileSync(
  fileURLToPath(new URL('./ui/ownership-transfer-sheet.tsx', import.meta.url)),
  'utf8',
);

test('ownership transfer is a compact native destination with progressively disclosed candidate review', () => {
  assert.match(sheet, /<NativeSheet/);
  assert.match(sheet, /compact/);
  assert.match(sheet, /<SettingsNavigationRow/);
  assert.match(sheet, /<SettingsSection/);
  assert.match(sheet, /setScreen\('candidates'\)/);
  assert.match(sheet, /setScreen\('review'\)/);
  assert.match(sheet, /pathOwnership\.recipientBecomesCreator/);
  assert.match(sheet, /pathOwnership\.creatorBecomesAdministrator/);
  assert.match(sheet, /pathOwnership\.noChangeUntilAccepted/);
});

test('ownership transfer renders accessible loading, empty, error, pagination, and busy states', () => {
  assert.match(sheet, /accessibilityRole="progressbar"/);
  assert.match(sheet, /<NativeContentUnavailable/);
  assert.match(sheet, /accessibilityRole="alert"/);
  assert.match(sheet, /candidates\.nextCursor/);
  assert.match(sheet, /onLoadMoreCandidates/);
  assert.match(sheet, /accessibilityState=\{\{ disabled: busy, selected/);
  assert.match(sheet, /disabled=\{busy\}/);
});

test('pending ownership transfer offers only role-appropriate compact native actions', () => {
  assert.match(sheet, /pending\.viewerRole === 'creator'/);
  assert.match(sheet, /onCancelPending/);
  assert.match(sheet, /pending\.viewerRole === 'recipient'/);
  assert.match(sheet, /onAcceptPending/);
  assert.match(sheet, /onDeclinePending/);
  assert.match(sheet, /<SettingsActionRow/);
  assert.match(sheet, /expirationSummary/);
});

test('archived pending transfers disable mutations without trapping the native sheet', () => {
  assert.match(sheet, /actionsDisabled\?: boolean/);
  assert.match(sheet, /disabled=\{busy \|\| actionsDisabled\}/);
  assert.match(sheet, /dismissible=\{!busy\}/);
  assert.match(sheet, /label: i18n\.t\('common\.done'\)/);
  assert.doesNotMatch(sheet, /dismissible=\{!busy && !actionsDisabled\}/);
});

test('sheet-owned failures remain visible and announced inside the native modal', () => {
  assert.match(sheet, /errorText\?: string/);
  assert.match(sheet, /errorText \? <View accessibilityRole="alert"/);
  assert.match(sheet, /<Text style=\{styles\.error\}>\{errorText\}<\/Text>/);
});
