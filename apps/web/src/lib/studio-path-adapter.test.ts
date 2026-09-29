const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { trackingFromAPI } from './studio/paths/adapters/api-path-repository';

test('rejects a running timer with no identity rather than offering an incorrect Start action', () => {
  assert.throws(() => trackingFromAPI({ running: true, accumulatedSeconds: 30 }), /path_request_failed/);
});
test('maps persisted totals separately from session time and rejects inverted periods', () => {
  const dto = { running: true, accumulatedSeconds: 120, timer: { id: 'session', pathId: 'path', occurrenceTimeZone: utcTimeZone, startedAt: '2026-09-29T12:00:00Z' } };
  const mapped = trackingFromAPI(dto);
  assert.equal(mapped.savedTotalSeconds, 120);
  assert.equal(mapped.activeSession?.startedAt, Date.parse(dto.timer.startedAt));
  assert.throws(() => trackingFromAPI({ ...dto, intervalProgress: {
    accumulatedSeconds: 10, targetSeconds: 60, startedAt: '2026-09-30T00:00:00Z', endedAt: '2026-09-29T00:00:00Z',
  } }), /path_request_failed/);
});
