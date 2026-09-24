import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const managePath = readFileSync(
  fileURLToPath(new URL('./ui/path-goal-management-form.tsx', import.meta.url)),
  'utf8',
);
const page = readFileSync(
  fileURLToPath(new URL('../app/index.tsx', import.meta.url)),
  'utf8',
);

test('native Path rename is a focused compact Manage Path destination with bounded input', () => {
  assert.match(managePath, /<NativeSheet[\s\S]*compact[\s\S]*visible=\{visible\}/);
  assert.match(managePath, /<SettingsSection/);
  assert.match(managePath, /setScreen\('name'\)/);
  assert.match(managePath, /derivePathRenamePresentation/);
  assert.match(managePath, /pathRename\.explanation/);
  assert.match(managePath, /accessibilityLabel=\{i18n\.t\('pathRename\.nameLabel'\)\}/);
  assert.match(managePath, /maxLength=\{100\}/);
  assert.match(managePath, /onSubmitEditing=\{\(\) => \{[\s\S]*if \(renamePresentation\.canSave\) onRenameSave\(\)/);
  assert.match(managePath, /disabled:\s*managementBusy \|\| !renamePresentation\.canSave/);
  assert.match(managePath, /pathRename\.saved/);
  assert.match(managePath, /pathRename\.invalid/);
  assert.doesNotMatch(managePath, /pathRename\.noChanges/);
  assert.doesNotMatch(managePath, /<StatusBanner[^>]*pathRename/);
  assert.doesNotMatch(managePath, /Alert\./);
});

test('native Path details capability-gate a serialized authoritative rename workflow', () => {
  assert.doesNotMatch(page, /label:\s*i18n\.t\('pathRename\.action'\)/);
  assert.match(page, /selectedCapabilities\.manageGoals \|\| selectedCapabilities\.renamePath \|\| selectedCapabilities\.manageLifecycle/);
  assert.match(page, /function openPathManagement\(path: SessionPath\)[\s\S]*capabilities\.renamePath/);
  assert.match(page, /reviewPathRename\(path, pathRenameName\)/);
  assert.match(page, /pathRenameOperations\.submit\([\s\S]*\.renamePath\(requestedPathID, body, idempotencyKey\)/);
  assert.match(page, /applyPathRenameResult\(\{[\s\S]*paths: current\.profile\.paths,[\s\S]*selectedPath: selected,[\s\S]*\}, result\.path\)/);
  assert.match(page, /profile:\s*\{[\s\S]*\.\.\.current\.profile,[\s\S]*paths: applied\.paths/);
  assert.match(page, /function closePathManagement\(dirty = false\)[\s\S]*resetPathRename\(\)/);
  assert.match(page, /<PathGoalManagementForm[\s\S]*onRenameSave=\{\(\) => void submitPathRename\(\)\}/);
  assert.match(page, /async function submitPathRename\(\)[\s\S]*goalManagementBusy[\s\S]*goalManagementReview/);
  assert.match(page, /async function confirmPathGoalChanges\(\)[\s\S]*pathRenameBusy/);
});
