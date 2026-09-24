import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');
const people = readFileSync(new URL('./path-member-access.svelte', import.meta.url), 'utf8');
const markup = (people.split('</script>')[1] ?? people).split('<style>')[0] ?? people;

test('every current Path member can open the dedicated People destination', () => {
  assert.match(page, /i18n\.t\('pathMembers\.heading'\)/);
  assert.match(page, /onclick=\{\(\) => void openPathMembers\(\)\}/);
  assert.doesNotMatch(
    page,
    /\{#if selectedPath\.capabilities\.manageMembers[^}]*\}[\s\S]{0,240}onclick=\{\(\) => void openPathMembers\(\)\}/,
  );
  assert.doesNotMatch(page, /function openPathMembers[\s\S]{0,500}capabilities\.manageMembers/);
});

test('People uses compact rows leading to Path-scoped member detail, never profiles', () => {
  assert.match(markup, /class="member-list"/);
  assert.match(markup, /member\.displayName/);
  assert.match(markup, /@\{member\.username\}/);
  assert.match(markup, /member\.role/);
  assert.match(markup, /onSelect\(member\)/);
  assert.match(markup, /selected/);
  assert.doesNotMatch(markup, /<div class="member-row protected"/);
  assert.doesNotMatch(markup, /href=[^>]*profile|\/profile\/|openProfile|selectedProfile/);
});

test('Path-scoped member detail conditionally offers Unblock and Remove without an incomplete Leave shortcut', () => {
  assert.match(people, /export let onUnblock/);
  assert.match(people, /export let onRemove/);
  assert.doesNotMatch(people, /export let onLeave|pathLeave\.action/);
  assert.match(markup, /selected\.blockedByViewer[\s\S]*blocking\.unblock/);
  assert.match(markup, /selected\.canRemove[\s\S]*pathMembers\.remove/);
  assert.doesNotMatch(markup, /blocksViewer|blockedByTarget/);
});

test('web orchestration reuses authoritative mutations with operation-specific feedback', () => {
  assert.match(page, /async function unblockPathMember/);
  assert.match(page, /unblockAccount\(member\.userId,\s*crypto\.randomUUID\(\)\)/);
  assert.match(page, /removeSelectedPathMember/);
  assert.match(page, /pathMemberUnblockBusy = true/);
  assert.match(page, /pathMemberUnblockError = true/);
  assert.match(markup, /unblockBusy[\s\S]*blocking\.unblockUnavailable/);
  assert.match(markup, /disabled=\{removalBusy \|\| unblockBusy \|\| !selected\.canRemove\}/);
  assert.match(page, /blockedByViewer[\s\S]*pathMembers/);
});
