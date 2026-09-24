import assert from 'node:assert/strict';
import test from 'node:test';
import { activityEditForm, activityValidationNow, groupActivitiesNewestFirst, mergeActivityHistory, mergeRevisionHistory, ownsActivity, removeActivity, type ActivityHistoryItem } from './path-activity-history.js';

function detail(id: string, startedAt: string, timeZone = 'Etc/UTC', participantId = 'owner-1'): ActivityHistoryItem {
  return {
    version: 1,
    activity: {
      id,
      pathId: 'path-1',
      participantId,
      startedAt,
      endedAt: new Date(Date.parse(startedAt) + 60_000).toISOString(),
      durationSeconds: 60,
      occurrenceTimeZone: timeZone,
      createdAt: startedAt,
      updatedAt: startedAt,
    },
  };
}

test('Path history is grouped by retained local calendar day and ordered newest first', () => {
  const groups = groupActivitiesNewestFirst([
    detail('older', '2026-07-21T23:30:00Z', 'America/New_York'),
    detail('newest', '2026-07-23T00:30:00Z', 'America/New_York'),
    detail('middle', '2026-07-22T16:00:00Z', 'America/New_York'),
  ]);

  assert.deepEqual(groups.map((group) => [group.localDate, group.items.map((item) => item.activity.id)]), [
    ['2026-07-22', ['newest', 'middle']],
    ['2026-07-21', ['older']],
  ]);
});

test('mixed retained timezones still produce one newest-first section per local day', () => {
  const groups = groupActivitiesNewestFirst([
    detail('new-york-late', '2026-01-02T01:00:00Z', 'America/New_York'),
    detail('tokyo', '2026-01-01T23:00:00Z', 'Asia/Tokyo'),
    detail('new-york-early', '2026-01-01T20:00:00Z', 'America/New_York'),
  ]);
  assert.deepEqual(groups.map((group) => [group.localDate, group.items.map((item) => item.activity.id)]), [
    ['2026-01-02', ['tokyo']],
    ['2026-01-01', ['new-york-late', 'new-york-early']],
  ]);
});

test('continuation activity pages append in order and deduplicate stable IDs while initial pages replace', () => {
  const first = [detail('newest', '2026-07-23T12:00:00Z'), detail('overlap', '2026-07-22T12:00:00Z')];
  const continued = mergeActivityHistory(first, [
    detail('overlap', '2026-07-22T12:00:00Z'),
    detail('oldest', '2026-07-21T12:00:00Z'),
  ]);
  assert.deepEqual(continued.map((item) => item.activity.id), ['newest', 'overlap', 'oldest']);
  assert.deepEqual(mergeActivityHistory(continued, [detail('replacement', '2026-07-24T12:00:00Z')], true).map((item) => item.activity.id), ['replacement']);
});

test('activity removal matches only the exact stable ID and preserves unrelated order', () => {
  const history = [
    detail('activity-1', '2026-07-23T12:00:00Z'),
    detail('activity-10', '2026-07-22T12:00:00Z'),
    detail('activity-2', '2026-07-21T12:00:00Z'),
  ];

  assert.deepEqual(
    removeActivity(history, 'activity-1').map((item) => item.activity.id),
    ['activity-10', 'activity-2'],
  );
  const unchanged = removeActivity(history, 'missing');
  assert.strictEqual(unchanged, history);
  assert.deepEqual(unchanged.map((item) => item.activity.id), ['activity-1', 'activity-10', 'activity-2']);
  assert.deepEqual(history.map((item) => item.activity.id), ['activity-1', 'activity-10', 'activity-2']);
});

test('continuation revision pages append and deduplicate stable versions while initial pages replace', () => {
  assert.deepEqual(
    mergeRevisionHistory([{ version: 4 }, { version: 3 }], [{ version: 3 }, { version: 2 }]),
    [{ version: 4 }, { version: 3 }, { version: 2 }],
  );
  assert.deepEqual(mergeRevisionHistory([{ version: 4 }], [{ version: 1 }], true), [{ version: 1 }]);
});

test('owner edit state is seeded from the retained instant and occurrence timezone', () => {
  const timerCreated = detail('timer-created', '2026-11-01T06:30:00Z', 'America/New_York');
  timerCreated.activity.durationSeconds = 3600;
  timerCreated.activity.note = 'Retained note';

  assert.deepEqual(activityEditForm(timerCreated.activity), {
    localDate: '2026-11-01',
    localTime: '01:30:00',
    durationSeconds: '3600',
    occurrenceTouched: true,
  });
  assert.equal(ownsActivity(timerCreated.activity, 'owner-1'), true);
  assert.equal(ownsActivity(timerCreated.activity, 'viewer-2'), false);
});

test('edit validation keeps the retained occurrence timezone after the profile timezone changes', () => {
  assert.deepEqual(
    activityValidationNow('2026-07-23T16:00:00Z', 'Europe/Paris', 30_000, 'America/New_York'),
    {
      localDate: '2026-07-23',
      localTime: '12:00:30',
      currentInstant: '2026-07-23T16:00:30.000Z',
      timeZone: 'America/New_York',
    },
  );
  assert.equal(
    activityValidationNow('2026-07-23T16:00:00Z', 'Europe/Paris', 0).timeZone,
    'Europe/Paris',
  );
});
