import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createTimeZoneChangeIntentCoordinator,
  filterTimeZones,
  timeZonePreferenceFromAPI,
} from './time-zone-settings';

test('authoritative time-zone preferences require one valid IANA zone and effective instant', () => {
  const fixedOffsetZone = ['UTC', '-04:00'].join('');
  const utcZone = ['Etc', 'UTC'].join('/');
  assert.deepEqual(timeZonePreferenceFromAPI({
    changed: false,
    effectiveAt: '2026-08-03T12:00:00Z',
    timeZone: 'America/New_York',
  }), {
    changed: false,
    effectiveAt: '2026-08-03T12:00:00.000Z',
    timeZone: 'America/New_York',
  });

  for (const invalid of [
    null,
    { changed: false, effectiveAt: '2026-08-03T12:00:00Z', timeZone: fixedOffsetZone },
    { changed: false, effectiveAt: 'not-an-instant', timeZone: utcZone },
    { changed: 'no', effectiveAt: '2026-08-03T12:00:00Z', timeZone: utcZone },
    { changed: false, effectiveAt: '2026-08-03T12:00:00Z', extra: true, timeZone: utcZone },
  ]) assert.throws(() => timeZonePreferenceFromAPI(invalid));
});

test('time-zone search is case-insensitive, stable, and keeps the configured zone available', () => {
  const zones = ['Europe/London', 'America/Chicago', 'America/New_York', 'Asia/Tokyo'];
  assert.deepEqual(filterTimeZones(zones, 'america', 'Europe/London'), [
    'America/Chicago',
    'America/New_York',
  ]);
  assert.deepEqual(filterTimeZones(zones, '', 'Pacific/Kiritimati'), [
    'Pacific/Kiritimati',
    'America/Chicago',
    'America/New_York',
    'Asia/Tokyo',
    'Europe/London',
  ]);
});

test('confirmed retries retain one reviewed-zone mutation identity until completion', () => {
  let next = 0;
  const coordinator = createTimeZoneChangeIntentCoordinator(() => `key-${++next}`);
  const first = coordinator.freeze('America/New_York', 'Europe/London');
  assert.deepEqual(first, {
    confirmed: true,
    idempotencyKey: 'key-1',
    proposedTimeZone: 'Europe/London',
    reviewedTimeZone: 'America/New_York',
  });
  assert.equal(coordinator.freeze('America/New_York', 'Europe/London'), first);
  assert.notEqual(coordinator.freeze('America/New_York', 'Asia/Tokyo'), first);
  coordinator.complete(coordinator.freeze('America/New_York', 'Asia/Tokyo'));
  assert.equal(coordinator.freeze('America/New_York', 'Asia/Tokyo').idempotencyKey, 'key-3');
});
