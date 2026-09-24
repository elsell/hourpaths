import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildGoalDraft,
  goalFormState,
  type DurationFields,
  type GoalFormState,
} from './path-goals';

const duration = (hours = '0', minutes = '0', seconds = '0'): DurationFields => ({
  hours,
  minutes,
  seconds,
});

const base = (): GoalFormState => ({
  intervalEnabled: false,
  intervalDuration: duration(),
  recurrence: 'daily',
  customAlignment: false,
  alignmentValue: '',
  yearlyMonth: '1',
  yearlyDay: '1',
  overallEnabled: false,
  overallDuration: duration(),
});

test('disabled goals are omitted so name-only Path creation stays quick', () => {
  assert.deepEqual(buildGoalDraft(base()), { ok: true, goals: {} });
});

test('interval and overall goals are independently optional', () => {
  const interval = { ...base(), intervalEnabled: true, intervalDuration: duration('1', '2', '3') };
  assert.deepEqual(buildGoalDraft(interval), {
    ok: true,
    goals: { intervalGoal: { targetSeconds: 3723, recurrence: 'daily' } },
  });

  const overall = { ...base(), overallEnabled: true, overallDuration: duration('10') };
  assert.deepEqual(buildGoalDraft(overall), {
    ok: true,
    goals: { overallTarget: { targetSeconds: 36_000 } },
  });

  assert.deepEqual(buildGoalDraft({ ...interval, overallEnabled: true, overallDuration: duration('10') }), {
    ok: true,
    goals: {
      intervalGoal: { targetSeconds: 3723, recurrence: 'daily' },
      overallTarget: { targetSeconds: 36_000 },
    },
  });
});

test('enabled goals require positive whole-second durations', () => {
  for (const intervalDuration of [
    duration(),
    duration('1.5'),
    duration('-1'),
    duration('0', '60'),
    duration('0', '0', '60'),
    duration('9007199254740991'),
  ]) {
    assert.deepEqual(
      buildGoalDraft({ ...base(), intervalEnabled: true, intervalDuration }),
      { ok: false, reason: 'duration' },
    );
  }
});

test('custom alignment emits only the field required by each recurrence', () => {
  const cases = [
    ['hourly', '17', { minute: 17 }],
    ['daily', '8', { hour: 8 }],
    ['weekly', '7', { isoWeekday: 7 }],
    ['monthly', '31', { day: 31 }],
  ] as const;

  for (const [recurrence, alignmentValue, alignment] of cases) {
    assert.deepEqual(buildGoalDraft({
      ...base(),
      intervalEnabled: true,
      intervalDuration: duration('1'),
      recurrence,
      customAlignment: true,
      alignmentValue,
    }), {
      ok: true,
      goals: { intervalGoal: { targetSeconds: 3600, recurrence, alignment } },
    });
  }

  assert.deepEqual(buildGoalDraft({
    ...base(),
    intervalEnabled: true,
    intervalDuration: duration('1'),
    recurrence: 'yearly',
    customAlignment: true,
    yearlyMonth: '2',
    yearlyDay: '29',
  }), {
    ok: true,
    goals: {
      intervalGoal: {
        targetSeconds: 3600,
        recurrence: 'yearly',
        alignment: { month: 2, day: 29 },
      },
    },
  });
});

test('custom alignment rejects out-of-range and impossible calendar values', () => {
  const invalid = [
    { recurrence: 'hourly' as const, alignmentValue: '60' },
    { recurrence: 'daily' as const, alignmentValue: '24' },
    { recurrence: 'weekly' as const, alignmentValue: '0' },
    { recurrence: 'monthly' as const, alignmentValue: '32' },
  ];
  for (const fields of invalid) {
    assert.deepEqual(buildGoalDraft({
      ...base(),
      intervalEnabled: true,
      intervalDuration: duration('1'),
      customAlignment: true,
      ...fields,
    }), { ok: false, reason: 'alignment' });
  }

  assert.deepEqual(buildGoalDraft({
    ...base(),
    intervalEnabled: true,
    intervalDuration: duration('1'),
    recurrence: 'yearly',
    customAlignment: true,
    yearlyMonth: '4',
    yearlyDay: '31',
  }), { ok: false, reason: 'alignment' });
});

test('stored interval goals seed exact editable duration and canonical alignment fields', () => {
  const cases = [
    {
      goal: { targetSeconds: 3_723, recurrence: 'hourly' as const, alignment: { minute: 17 } },
      form: {
        recurrence: 'hourly', alignmentValue: '17',
        yearlyMonth: '1', yearlyDay: '1',
      },
    },
    {
      goal: { targetSeconds: 3_723, recurrence: 'daily' as const, alignment: { hour: 8 } },
      form: {
        recurrence: 'daily', alignmentValue: '8',
        yearlyMonth: '1', yearlyDay: '1',
      },
    },
    {
      goal: { targetSeconds: 3_723, recurrence: 'weekly' as const, alignment: { isoWeekday: 7 } },
      form: {
        recurrence: 'weekly', alignmentValue: '7',
        yearlyMonth: '1', yearlyDay: '1',
      },
    },
    {
      goal: { targetSeconds: 3_723, recurrence: 'monthly' as const, alignment: { day: 31 } },
      form: {
        recurrence: 'monthly', alignmentValue: '31',
        yearlyMonth: '1', yearlyDay: '1',
      },
    },
    {
      goal: { targetSeconds: 3_723, recurrence: 'yearly' as const, alignment: { month: 2, day: 29 } },
      form: {
        recurrence: 'yearly', alignmentValue: '',
        yearlyMonth: '2', yearlyDay: '29',
      },
    },
  ];

  for (const { goal, form: expected } of cases) {
    const form = goalFormState({
      intervalGoal: goal,
      overallTarget: { targetSeconds: 90_061 },
    });
    assert.deepEqual(form, {
      intervalEnabled: true,
      intervalDuration: duration('1', '2', '3'),
      recurrence: expected.recurrence,
      customAlignment: true,
      alignmentValue: expected.alignmentValue,
      yearlyMonth: expected.yearlyMonth,
      yearlyDay: expected.yearlyDay,
      overallEnabled: true,
      overallDuration: duration('25', '1', '1'),
    });
  }
});

test('stored goals round-trip to an exact canonical update and can be explicitly removed', () => {
  const stored = {
    intervalGoal: {
      targetSeconds: 1_800,
      recurrence: 'weekly' as const,
      alignment: { isoWeekday: 1 },
    },
    overallTarget: { targetSeconds: 360_000 },
  };
  const seeded = goalFormState(stored);

  assert.deepEqual(buildGoalDraft(seeded), { ok: true, goals: stored });
  assert.deepEqual(buildGoalDraft({
    ...seeded,
    intervalEnabled: false,
    overallEnabled: false,
  }), { ok: true, goals: {} });
  assert.deepEqual(stored, {
    intervalGoal: {
      targetSeconds: 1_800,
      recurrence: 'weekly',
      alignment: { isoWeekday: 1 },
    },
    overallTarget: { targetSeconds: 360_000 },
  });
});
