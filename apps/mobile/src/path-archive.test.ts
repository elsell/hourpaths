import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

const page = readFileSync(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
const pathDetailView = readFileSync(fileURLToPath(new URL('./ui/path-detail-view.tsx', import.meta.url)), 'utf8');
const archiveSheet = readFileSync(fileURLToPath(new URL('./ui/path-archive-confirmation-sheet.tsx', import.meta.url)), 'utf8');

function sourceSection(start: string, end: string): string {
  const startIndex = page.indexOf(start);
  assert.notEqual(startIndex, -1, `missing source section ${start}`);
  const endIndex = page.indexOf(end, startIndex + start.length);
  assert.notEqual(endIndex, -1, `missing source section terminator ${end}`);
  return page.slice(startIndex, endIndex);
}

test('mobile Home separates active and archived Paths while archived detail stays reachable', () => {
  assert.match(page, /const \[archivedPathsOpen, setArchivedPathsOpen\] = useState\(false\)/);
  assert.match(page, /homeSections\.active\.map\(renderHomePath\)/);
  assert.match(page, /homeSections\.trackable\.map\(renderHomePath\)/);
  assert.match(page, /ownedHomeDestination\.profile\.archivedPaths\.map\(\(path\)/);
  assert.match(page, /<HomeHeaderActions[\s\S]*mode=\{archivedPathsOpen \? 'archived' : 'active'\}/);
  assert.match(page, /ownedHomeDestination\.profile\.archivedPaths\.find\(\(path\) => path\.id === selectedPathID\)/);
});

test('archived Path detail is read-only but preserves history access', () => {
  const detail = sourceSection('{selectedPath ? <NativeRouteSource', '{selectedCapabilities.manageLifecycle && pathArchiveReview ?');
  assert.match(detail, /archived=\{Boolean\(selectedPath\.archivedAt\)\}/);
  assert.match(pathDetailView, /archived[\s\S]*accessibilityRole="alert"[\s\S]*i18n\.t\('pathArchive\.readOnly'\)/);
  assert.match(detail, /selectedCapabilities\.manageGoals/);
  assert.match(detail, /selectedCapabilities\.trackTime/);
  assert.match(pathDetailView, /!archived && canTrackTime \? <NativePrimaryButton/);
  assert.match(detail, /openActivityHistory\(selectedPath\.id\)/);
});

test('archive and unarchive require explicit confirmation and retain one request body and key across safe retries', () => {
  assert.match(page, /const pathArchiveOperations = createPathArchiveOperationOwner\(\(\) => Crypto\.randomUUID\(\)\)/);
  assert.match(page, /setPathArchiveReview\(reviewPathArchiveChange\(path\)\)/);
  const confirm = sourceSection('async function confirmPathArchiveChange()', 'function cancelPathArchiveChange');
  assert.match(confirm, /pathArchiveOperations\.submit\(review, true,/);
  assert.match(confirm, /\.setPathArchiveState\(requestedPathID, body, idempotencyKey\)/);
  assert.match(confirm, /applyPathArchiveResult\(/);
  const review = sourceSection('{selectedCapabilities.manageLifecycle && pathArchiveReview ? <PathArchiveConfirmationSheet', '{(selectedCapabilities.manageGoals || selectedCapabilities.renamePath || selectedCapabilities.manageLifecycle ||');
  assert.match(archiveSheet, /archived \? 'pathArchive\.unarchiveWarning' : 'pathArchive\.warning'/);
  assert.match(review, /confirmPathArchiveChange\(\)/);
  assert.match(review, /cancelPathArchiveChange/);
  assert.match(archiveSheet, /dismissible=\{!busy\}/);
});

test('archive projection stops selected timer state and unarchive never restarts it', () => {
  const confirm = sourceSection('async function confirmPathArchiveChange()', 'function cancelPathArchiveChange');
  assert.match(confirm, /if \(!body\.archived && effectivePathCapabilities\(path\)\.trackTime\)[\s\S]*\.currentTimer\(requestedPathID\)/);
  assert.match(confirm, /applyPathArchiveResult\(\{[\s\S]*activePaths: current\.profile\.paths,[\s\S]*archivedPaths: current\.profile\.archivedPaths,[\s\S]*timerStates: unarchivedTimer/);
  assert.match(confirm, /unarchivedTimer[\s\S]*running: false,[\s\S]*timer: undefined/);
  assert.match(confirm, /paths: applied\.activePaths/);
  assert.match(confirm, /archivedPaths: applied\.archivedPaths/);
  assert.match(confirm, /timers: applied\.timerStates/);
});
