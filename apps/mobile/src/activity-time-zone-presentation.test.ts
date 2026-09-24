import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath, URL } from 'node:url';
import test from 'node:test';

const detailView = readFileSync(
  fileURLToPath(new URL('./ui/activity-detail-view.tsx', import.meta.url)),
  'utf8',
);

test('activity detail explicitly presents its retained IANA time zone in the compact detail rows', () => {
  const currentActivity = detailView.slice(
    detailView.indexOf('const ownsActivity'),
    detailView.indexOf('{!archived && ownsActivity'),
  );

  assert.match(currentActivity, /<DetailRow/);
  assert.match(currentActivity, /activity\.occurrenceTimeZone/);
  assert.match(currentActivity, /activity\.activity\.occurrenceTimeZone/);
});

test('every retained revision explicitly presents its own IANA time zone', () => {
  const revisionRows = detailView.slice(
    detailView.indexOf('revisions.map'),
    detailView.indexOf('{revisionPresentation.showError'),
  );

  assert.match(revisionRows, /<DetailRow/);
  assert.match(revisionRows, /activity\.occurrenceTimeZone/);
  assert.match(revisionRows, /revision\.occurrenceTimeZone/);
});
