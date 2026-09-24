import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('web presents complete destructive Path deletion and a local no-op cancel', () => {
  assert.match(page, /reviewPathDeletion\(selectedPath\)/);
  assert.match(page, /pathDelete\.warning/);
  assert.match(page, /pathDelete\.timerWarning/);
  assert.match(page, /pathDelete\.archiveAlternative/);
  assert.match(page, /pathDeletionOperations\.submit\(review, true/);
  assert.match(page, /\.deletePath\(pathID, body, idempotencyKey\)/);
  assert.match(page, /applyPathDeletionResult/);
  const cancel = page.slice(page.indexOf('function cancelPathDeletion'), page.indexOf('async function confirmPathDeletion'));
  assert.doesNotMatch(cancel, /deletePath|applyPathDeletionResult/);
});

test('web removes the deleted Path projection and does not render a navigable deletion notice', () => {
  assert.match(page, /selectedPath = null/);
  assert.match(page, /notification\.type === 'path_deleted'/);
  assert.match(page, /notificationPresentationMessageKey/);
});
