import assert from 'node:assert/strict';
import test from 'node:test';
import type { SessionPath } from '@hourpaths/api-client';
import {
  buildPathCreateDraft,
  buildPathGoalUpdateDraft,
  initialPathGoalForm,
  pathGoalFormFromPath,
  pathRecurrences,
} from './path-goals';
import {
  durationInputForSeconds,
  durationSecondsForInput,
  pathAlignmentIsCustomized,
  resetPathAlignment,
} from './ui/path-create-presentation';

const managerCapabilities = {
  leavePath: false,
  trackTime: true,
  inviteMembers: false,
  manageGoals: true,
  manageLifecycle: false,
  manageMembers: false,
  manageVisibility: false,
  renamePath: true,
  transferOwnership: false,
} as const;

test('Path goal form exposes exactly the five specified calendar recurrences', () => {
  assert.deepEqual(pathRecurrences, ['hourly', 'daily', 'weekly', 'monthly', 'yearly']);
});

test('Create Path presents human duration units without changing whole-second serialization', () => {
  assert.deepEqual(durationInputForSeconds('', 'minutes'), { unit: 'minutes', value: '' });
  assert.deepEqual(durationInputForSeconds('7200', 'minutes'), { unit: 'hours', value: '2' });
  assert.deepEqual(durationInputForSeconds('600', 'hours'), { unit: 'minutes', value: '10' });
  assert.deepEqual(durationInputForSeconds('61', 'minutes'), { unit: 'seconds', value: '61' });
  assert.equal(durationSecondsForInput('2', 'hours'), '7200');
  assert.equal(durationSecondsForInput('10', 'minutes'), '600');
  assert.equal(durationSecondsForInput('45', 'seconds'), '45');
  assert.equal(durationSecondsForInput('', 'hours'), '');
});

test('Create Path can progressively disclose and reset customized calendar alignment', () => {
  const daily = { ...initialPathGoalForm(), recurrence: 'daily' as const, dailyHour: '6' };
  assert.equal(pathAlignmentIsCustomized(daily), true);
  assert.deepEqual(resetPathAlignment(daily), { dailyHour: '0' });
  assert.equal(pathAlignmentIsCustomized({ ...daily, dailyHour: '0' }), false);

  const weekly = { ...initialPathGoalForm(), recurrence: 'weekly' as const, weeklyISOWeekday: '1' };
  assert.equal(pathAlignmentIsCustomized(weekly), true);
  assert.deepEqual(resetPathAlignment(weekly), { weeklyISOWeekday: '' });
});

test('name-only Path creation remains the initial and valid fast path', () => {
  const result = buildPathCreateDraft('x', initialPathGoalForm());
  assert.deepEqual(result, { kind: 'valid', draft: { name: 'x' } });
});

test('Path creation serializes the explicitly reviewed visibility', () => {
  const result = buildPathCreateDraft('x', initialPathGoalForm(), 'followers');
  assert.deepEqual(result, {
    kind: 'valid',
    draft: { name: 'x', visibility: 'followers' },
  });
});

test('interval and overall goals are independently optional', () => {
  const interval = buildPathCreateDraft('x', {
    ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds: '600', recurrence: 'daily', dailyHour: '6',
  });
  assert.deepEqual(interval, {
    kind: 'valid',
    draft: { name: 'x', intervalGoal: { targetSeconds: 600, recurrence: 'daily', alignment: { hour: 6 } } },
  });

  const overall = buildPathCreateDraft('x', {
    ...initialPathGoalForm(), overallEnabled: true, overallSeconds: '360000',
  });
  assert.deepEqual(overall, {
    kind: 'valid', draft: { name: 'x', overallTarget: { targetSeconds: 360000 } },
  });

  const both = buildPathCreateDraft('x', {
    ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds: '60', overallEnabled: true, overallSeconds: '3600',
  });
  assert.deepEqual(both, {
    kind: 'valid',
    draft: {
      name: 'x',
      intervalGoal: { targetSeconds: 60, recurrence: 'hourly', alignment: { minute: 0 } },
      overallTarget: { targetSeconds: 3600 },
    },
  });
});

test('each recurrence sends its visible alignment and weekly can use the profile default', () => {
  const cases = [
    ['hourly', { minute: 45 }, { hourlyMinute: '45' }],
    ['daily', { hour: 18 }, { recurrence: 'daily', dailyHour: '18' }],
    ['weekly', { isoWeekday: 7 }, { recurrence: 'weekly', weeklyISOWeekday: '7' }],
    ['monthly', { day: 31 }, { recurrence: 'monthly', monthlyDay: '31' }],
    ['yearly', { month: 2, day: 29 }, { recurrence: 'yearly', yearlyMonth: '2', yearlyDay: '29' }],
  ] as const;
  for (const [recurrence, alignment, overrides] of cases) {
    const result = buildPathCreateDraft('x', {
      ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds: '60', recurrence, ...overrides,
    });
    assert.equal(result.kind, 'valid');
    if (result.kind === 'valid') assert.deepEqual(result.draft.intervalGoal?.alignment, alignment);
  }

  const profileDefault = buildPathCreateDraft('x', {
    ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds: '60', recurrence: 'weekly', weeklyISOWeekday: '',
  });
  assert.deepEqual(profileDefault, {
    kind: 'valid', draft: { name: 'x', intervalGoal: { targetSeconds: 60, recurrence: 'weekly' } },
  });
});

test('goal duration must be positive whole seconds and alignments must be calendar-valid', () => {
  for (const intervalSeconds of ['', '0', '-1', '1.5', '1e3']) {
    assert.deepEqual(buildPathCreateDraft('x', {
      ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds,
    }), { kind: 'invalid_duration' });
  }
  assert.deepEqual(buildPathCreateDraft('x', {
    ...initialPathGoalForm(), overallEnabled: true, overallSeconds: '0',
  }), { kind: 'invalid_duration' });
  assert.deepEqual(buildPathCreateDraft('x', {
    ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds: '1', hourlyMinute: '60',
  }), { kind: 'invalid_alignment' });
  assert.deepEqual(buildPathCreateDraft('x', {
    ...initialPathGoalForm(), intervalEnabled: true, intervalSeconds: '1', recurrence: 'yearly', yearlyMonth: '2', yearlyDay: '30',
  }), { kind: 'invalid_alignment' });
});

test('goal editing seeds the exact stored interval and overall configuration without mutating it', () => {
  const path: SessionPath = {
    id: 'path-id',
    name: 'Practice',
    visibility: 'private',
    capabilities: managerCapabilities,
    intervalGoal: {
      targetSeconds: 2_700,
      recurrence: 'daily',
      alignment: { hour: 18 },
    },
    overallTarget: { targetSeconds: 360_000 },
  };
  const before = structuredClone(path);

  assert.deepEqual(pathGoalFormFromPath(path), {
    ...initialPathGoalForm(),
    intervalEnabled: true,
    intervalSeconds: '2700',
    recurrence: 'daily',
    dailyHour: '18',
    overallEnabled: true,
    overallSeconds: '360000',
  });
  assert.deepEqual(path, before);
});

test('seeded update forms round-trip every recurrence-specific stored alignment', () => {
  const cases = [
    ['hourly', { minute: 45 }],
    ['daily', { hour: 18 }],
    ['weekly', { isoWeekday: 7 }],
    ['monthly', { day: 31 }],
    ['yearly', { month: 2, day: 29 }],
  ] as const;

  for (const [recurrence, alignment] of cases) {
    const path: SessionPath = {
      id: `${recurrence}-path`,
      name: 'Practice',
      visibility: 'private',
      capabilities: managerCapabilities,
      intervalGoal: { targetSeconds: 900, recurrence, alignment },
      overallTarget: { targetSeconds: 36_000 },
    };
    const before = structuredClone(path);

    assert.deepEqual(buildPathGoalUpdateDraft(pathGoalFormFromPath(path)), {
      kind: 'valid',
      draft: {
        intervalGoal: { targetSeconds: 900, recurrence, alignment },
        overallTarget: { targetSeconds: 36_000 },
      },
    });
    assert.deepEqual(path, before);
  }
});

test('disabling seeded goals serializes their removal without mutating the stored Path', () => {
  const path: SessionPath = {
    id: 'path-id',
    name: 'Practice',
    visibility: 'private',
    capabilities: managerCapabilities,
    intervalGoal: {
      targetSeconds: 3_600,
      recurrence: 'weekly',
      alignment: { isoWeekday: 1 },
    },
    overallTarget: { targetSeconds: 360_000 },
  };
  const before = structuredClone(path);
  const seeded = pathGoalFormFromPath(path);

  assert.deepEqual(buildPathGoalUpdateDraft({
    ...seeded,
    intervalEnabled: false,
    overallEnabled: false,
  }), {
    kind: 'valid',
    draft: {},
  });
  assert.deepEqual(path, before);
});
