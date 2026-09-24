import assert from 'node:assert/strict';
import test from 'node:test';
import {
  activityBelongsToProfile,
  activityEditSeed,
  activityWasEdited,
  appendUniqueActivities,
  appendUniqueRevisions,
  groupActivitiesByOccurrenceDay,
  newestActivitiesFirst,
  pagedCollectionPresentation,
  priorNoteForProfile,
  retainedCalendarDayDate,
  retainedCalendarDayTimeZone,
  type ActivityDetail,
  type ActivityRevision,
  type ManualActivityDefaults,
} from './activity-history';
import {
  localDateFromPicker,
  localTimeFromPicker,
  manualOccurrencePickerValue,
} from './ui/manual-occurrence-values';

const tokyoTimeZone = String.fromCharCode(69, 116, 99, 47, 71, 77, 84, 45, 57);
const universalTimeZone = String.fromCharCode(69, 116, 99, 47, 85, 84, 67);
const kiritimatiTimeZone = String.fromCharCode(80, 97, 99, 105, 102, 105, 99, 47, 75, 105, 114, 105, 116, 105, 109, 97, 116, 105);
const apiaTimeZone = String.fromCharCode(80, 97, 99, 105, 102, 105, 99, 47, 65, 112, 105, 97);

function detail(id: string, startedAt: string, participantId = 'profile-1'): ActivityDetail {
  return {
    version: 1,
    activity: {
      id, pathId: 'path-1', participantId, startedAt,
      endedAt: '2026-07-21T14:30:00.000Z', occurrenceTimeZone: 'America/New_York',
      durationSeconds: 1800, note: 'retained note', createdAt: startedAt, updatedAt: startedAt,
    },
  };
}

test('activity history is newest-first without mutating the API list', () => {
  const older = detail('older', '2026-07-20T12:00:00.000Z');
  const newer = detail('newer', '2026-07-21T12:00:00.000Z');
  const source = [older, newer];
  assert.deepEqual(newestActivitiesFirst(source).map((item) => item.activity.id), ['newer', 'older']);
  assert.deepEqual(source, [older, newer]);
});

test('history groups retained-timezone local days with newest sections and entries first', () => {
  const lateNewYork = detail('late-ny', '2026-07-22T03:30:00.000Z');
  const earlyNewYork = detail('early-ny', '2026-07-21T13:00:00.000Z');
  const tokyoNextDay = {
    ...detail('tokyo', '2026-07-21T16:00:00.000Z'),
    activity: { ...detail('tokyo', '2026-07-21T16:00:00.000Z').activity, occurrenceTimeZone: tokyoTimeZone },
  };
  assert.deepEqual(
    groupActivitiesByOccurrenceDay([earlyNewYork, tokyoNextDay, lateNewYork]).map((group) => ({
      localDate: group.localDate,
      ids: group.items.map((item) => item.activity.id),
    })),
    [
      { localDate: '2026-07-22', ids: ['tokyo'] },
      { localDate: '2026-07-21', ids: ['late-ny', 'early-ny'] },
    ],
  );
});

test('activity pages append by ID, replace duplicates, and preserve newest-first order', () => {
  const first = detail('first', '2026-07-20T12:00:00.000Z');
  const duplicate = { ...detail('first', '2026-07-20T12:00:00.000Z'), version: 2 };
  const newer = detail('newer', '2026-07-22T12:00:00.000Z');
  assert.deepEqual(
    appendUniqueActivities([first], [newer, duplicate]).map((item) => [item.activity.id, item.version]),
    [['newer', 1], ['first', 2]],
  );
});

test('revision pages append by version, replace duplicates, and sort newest version first', () => {
  const base = detail('activity', '2026-07-20T12:00:00.000Z').activity;
  const revision = (version: number, note: string): ActivityRevision => ({
    ...base, version, note, replacedAt: `2026-07-2${version}T12:00:00.000Z`,
  });
  assert.deepEqual(
    appendUniqueRevisions([revision(1, 'old')], [revision(2, 'newer'), revision(1, 'replacement')])
      .map((item) => [item.version, item.note]),
    [[2, 'newer'], [1, 'replacement']],
  );
});

test('only the activity participant may edit an activity', () => {
  assert.equal(activityBelongsToProfile(detail('mine', '2026-07-21T12:00:00.000Z'), 'profile-1'), true);
  assert.equal(activityBelongsToProfile(detail('theirs', '2026-07-21T12:00:00.000Z', 'profile-2'), 'profile-1'), false);
});

test('viewer-scoped history versions hide note-only edits from non-owners', () => {
  const original = detail('original', '2026-07-21T12:00:00.000Z');
  const ownerNoteOnlyProjection = { ...original, version: 2 };
  const otherViewerNoteOnlyProjection = {
    ...original,
    activity: { ...original.activity, note: undefined },
    version: 1,
  };
  const otherViewerPublicEditProjection = {
    ...otherViewerNoteOnlyProjection,
    version: 2,
  };
  assert.equal(activityWasEdited(original), false);
  assert.equal(activityWasEdited(ownerNoteOnlyProjection), true);
  assert.equal(activityWasEdited(otherViewerNoteOnlyProjection), false);
  assert.equal(activityWasEdited(otherViewerPublicEditProjection), true);
  const revision: ActivityRevision = {
    ...original.activity, version: 1, replacedAt: '2026-07-22T12:00:00.000Z', note: 'private prior note',
  };
  assert.equal(priorNoteForProfile(revision, 'profile-1'), 'private prior note');
  assert.equal(priorNoteForProfile(revision, 'profile-2'), null);
});

test('retained calendar-day headings survive UTC+14 and skipped device dates', () => {
  const originalTimeZone = process.env.TZ;
  try {
    const headingParts = (localDate: string) => Object.fromEntries(
      new Intl.DateTimeFormat('en', {
        day: '2-digit',
        month: '2-digit',
        timeZone: retainedCalendarDayTimeZone,
        year: 'numeric',
      }).formatToParts(retainedCalendarDayDate(localDate)).map((part) => [part.type, part.value]),
    );
    process.env.TZ = kiritimatiTimeZone;
    const kiritimatiHeading = headingParts('2026-01-15');
    assert.deepEqual(
      [kiritimatiHeading.year, kiritimatiHeading.month, kiritimatiHeading.day],
      ['2026', '01', '15'],
    );
    process.env.TZ = apiaTimeZone;
    const skippedDateHeading = headingParts('2011-12-30');
    assert.deepEqual(
      [skippedDateHeading.year, skippedDateHeading.month, skippedDateHeading.day],
      ['2011', '12', '30'],
    );
  } finally {
    process.env.TZ = originalTimeZone;
  }
});

test('paged collection presentation distinguishes loading, empty, retry, and busy pagination', () => {
  assert.deepEqual(pagedCollectionPresentation({
    busy: true, error: false, hasMore: false, itemCount: 0,
  }), {
    showEmpty: false, showError: false, showLoadMore: false, showLoading: true, showRetry: false,
  });
  assert.deepEqual(pagedCollectionPresentation({
    busy: false, error: false, hasMore: false, itemCount: 0,
  }), {
    showEmpty: true, showError: false, showLoadMore: false, showLoading: false, showRetry: false,
  });
  assert.deepEqual(pagedCollectionPresentation({
    busy: false, error: true, hasMore: true, itemCount: 3,
  }), {
    showEmpty: false, showError: true, showLoadMore: false, showLoading: false, showRetry: true,
  });
  assert.deepEqual(pagedCollectionPresentation({
    busy: true, error: false, hasMore: true, itemCount: 3,
  }), {
    showEmpty: false, showError: false, showLoadMore: true, showLoading: true, showRetry: false,
  });
});

test('edit seed retains the recorded instant, timezone, duration, and note', () => {
  const recorded = detail('manual-or-timer', '2026-01-15T15:05:00.000Z');
  const defaults: ManualActivityDefaults = {
    currentInstant: '2026-07-21T16:00:00.000Z', localDate: '2026-07-21',
    localStartTime: '12:00', timeZone: universalTimeZone,
  };
  const seed = activityEditSeed(recorded, defaults);
  assert.equal(seed.defaults.timeZone, 'America/New_York');
  assert.equal(seed.defaults.currentInstant, defaults.currentInstant);
  assert.deepEqual(seed.form, {
    localDate: '2026-01-15', localTime: '10:05:00', durationSeconds: '1800', occurrenceTouched: true,
  });
  assert.equal(seed.note, 'retained note');
});

test('native occurrence pickers preserve participant-local fields without applying the device timezone', () => {
  const pickerValue = manualOccurrencePickerValue('2026-11-01', '01:30:45');
  assert.equal(pickerValue?.toISOString(), '2026-11-01T01:30:45.000Z');
  assert.equal(localDateFromPicker(new Date('2026-07-04T23:59:00.000Z')), '2026-07-04');
  assert.equal(localTimeFromPicker(new Date('2026-07-04T23:59:07.000Z')), '23:59:00');
  assert.equal(manualOccurrencePickerValue('not-a-date', '01:30'), null);
  assert.equal(manualOccurrencePickerValue('2026-11-01', '25:30'), null);
});
