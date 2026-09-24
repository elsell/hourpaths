import assert from 'node:assert/strict';
import test from 'node:test';
import { manualActivityDraft, manualActivityDraftChanged } from './manual-activity-draft';

const form = {
  durationSeconds: '60',
  localDate: '2026-08-14',
  localTime: '09:30:00',
  occurrenceTouched: false,
};

test('manual activity dirty state clears after every field returns to its normalized baseline', () => {
  const baseline = manualActivityDraft(form, 'baseline');
  assert.equal(manualActivityDraftChanged(baseline, manualActivityDraft({ ...form, durationSeconds: '120' }, 'baseline')), true);
  assert.equal(manualActivityDraftChanged(baseline, manualActivityDraft({ ...form, durationSeconds: '060' }, 'baseline')), false);
  assert.equal(manualActivityDraftChanged(baseline, manualActivityDraft(form, 'changed')), true);
  assert.equal(manualActivityDraftChanged(baseline, manualActivityDraft(form, 'baseline')), false);
});
