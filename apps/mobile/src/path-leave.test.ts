import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const nativeMenu = readFileSync(fileURLToPath(new URL('./ui/path-header-menu.ios.tsx', import.meta.url)), 'utf8');
const nativeConfirmation = readFileSync(fileURLToPath(new URL('./ui/native-confirmation.ts', import.meta.url)), 'utf8');

test('PATH-09 capability drives native destructive route wiring', () => {
  assert.match(page, /selectedCapabilities\.leavePath \? \[\{/);
  assert.match(page, /destructive: true,[\s\S]*pathLeave\.action[\s\S]*rectangle\.portrait\.and\.arrow\.right/);
  assert.match(nativeMenu, /destructive=\{action\.destructive\}/);
  assert.match(page, /!effectivePathCapabilities\(path\)\.trackTime/);
});

test('participant leave uses a native three-button choice and a second destructive deletion confirmation', () => {
  assert.match(page, /presentNativePathLeaveChoice\(\{[\s\S]*pathLeave\.keepActivity[\s\S]*pathLeave\.deleteActivity/);
  assert.match(page, /onDelete:[\s\S]*reviewPathLeave\(path, false\)[\s\S]*presentNativeDestructiveConfirmation\([\s\S]*pathLeave\.deleteWarning/);
  assert.match(nativeConfirmation, /Alert\.alert\(title, message, \[/);
  assert.match(nativeConfirmation, /onPress: onKeep, style: 'default', text: keepLabel/);
  assert.match(nativeConfirmation, /onPress: onDelete, style: 'destructive', text: deleteLabel/);
  assert.match(nativeConfirmation, /onPress: onCancel, style: 'cancel', text: cancelLabel/);
  assert.doesNotMatch(nativeConfirmation, /ActionSheetIOS|Platform/);
  assert.match(page, /from '\.\.\/src\/ui\/native-confirmation'/);
});

test('PATH-09 confirmation uses the generated client and exact retained/deleted reconciliation', () => {
  assert.match(page, /pathLeaveOperations\.submit\(review, true/);
  assert.match(page, /\.leavePath\([\s\S]*requestedPathID, body, idempotencyKey/);
  assert.match(page, /applyPathLeaveResult/);
  assert.match(page, /setSelectedPathID\(null\)/);
  assert.match(page, /review\.retainActivity \? 'pathLeave\.completedRetained' : 'pathLeave\.completedDeleted'/);
});

test('PATH-09 mutation is account, session, Path, and current capability bound', () => {
  assert.match(page, /pathLeaveTarget\.current !== target/);
  assert.match(page, /current\.session !== target\.session/);
  assert.match(page, /current\.destination\.profile\.id !== target\.ownerID/);
  assert.match(page, /path\.name !== target\.pathName/);
  assert.match(page, /pathLeaveTarget\.current\.session\.token !== credential\.token\) resetPathLeave/);
  assert.match(page, /pathLeaveBusy \? <StatusBanner text=\{i18n\.t\('pathLeave\.leaving'\)\}/);
  assert.match(page, /pathLeaveErrorKey \? <StatusBanner[\s\S]*tone="error"/);
  assert.match(page, /if \(pathLeaveBusy\) return;[\s\S]*resetGoalManagement/);
  assert.match(page, /actionDisabled=\{pathLeaveBusy \|\|/);
});
