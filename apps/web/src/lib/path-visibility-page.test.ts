import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('Manage Path exposes creator-only effective visibility controls and no-op protection', () => {
  assert.match(page, /authenticatedProfileFromAPI/);
  assert.match(page, /selectedCapabilities\.manageVisibility/);
  assert.match(page, /pathVisibilityOptions\(profile\.profileVisibility\)/);
  assert.match(page, /reviewPathVisibilityChange\(visibilitySessionPath\(selectedPath\), pathVisibilityDraft, profile\.profileVisibility\)/);
  assert.match(page, /review\.kind === 'unchanged'/);
  assert.match(page, /pathVisibility\.current/);
  assert.match(page, /pathVisibility\.choiceLabel/);
  assert.match(page, /pathVisibility\.option\.\$\{visibility\}/);
});

test('broader visibility requires an accessible, explicit confirmation with complete scope copy', () => {
  assert.match(page, /review\.broader/);
  assert.match(page, /<dialog[\s\S]*?use:showModal[\s\S]*?role="alertdialog"[\s\S]*?path-visibility-confirmation-heading/);
  assert.match(page, /pathVisibility\.confirmation\.transition/);
  assert.match(page, /pathVisibility\.confirmation\.historyExposure/);
  assert.match(page, /pathVisibility\.confirmation\.unchangedScope/);
  assert.match(page, /confirmPathVisibility/);
});

test('visibility mutation is retry-safe, authoritative, and rejects stale session or Path completion', () => {
  assert.match(page, /createPathVisibilityOperationOwner\(\(\) => crypto\.randomUUID\(\)\)/);
  assert.match(page, /\.setPathVisibility\(requestedPathID, body, idempotencyKey\)/);
  assert.match(page, /applyPathVisibilityResult\(/);
  assert.match(page, /session !== current \|\| profile\?\.id !== ownerID \|\| selectedPath\?\.id !== pathID/);
  assert.match(page, /result\.kind === 'failed'/);
});

test('visibility conflict converges active and archived collections and clears inaccessible selection', () => {
  const reload = page.slice(page.indexOf('async function reloadVisibilityPath'), page.indexOf('async function submitPathVisibility'));
  assert.match(reload, /Promise\.all\(/);
  assert.match(reload, /\.paths\(\)/);
  assert.match(reload, /\.archivedPaths\(\)/);
  assert.match(reload, /paths = refreshedActive/);
  assert.match(reload, /archivedPaths = refreshedArchived\.items/);
  assert.match(reload, /refreshedActive\.find[\s\S]*refreshedArchived\.items\.find/);
  assert.match(reload, /if \(!authoritative\) \{\s*resetPathDetails\(\);\s*return;/);
  assert.match(reload, /!effectivePathCapabilities\(authoritative\)\.manageVisibility[\s\S]*resetGoalManagement\(\)/);
});

test('visibility-change notification shows new audience and Path-only scope', () => {
  assert.match(page, /pathVisibility: 'pathVisibility' in notification \? pathVisibilityLabel\(notification\.pathVisibility\) : ''/);
});
