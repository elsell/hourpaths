import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyGoalMutationResult,
  compareGoalConfigurations,
} from './index.js';

const currentGoals = () => ({
  intervalGoal: {
    targetSeconds: 60,
    recurrence: 'daily' as const,
    alignment: { hour: 6 },
  },
  overallTarget: { targetSeconds: 3_600 },
});
const managerCapabilities = {
  trackTime: true,
  renamePath: true,
  inviteMembers: true,
  manageGoals: true,
  manageLifecycle: false,
  transferOwnership: false,
};

test('goal comparison retains immutable old and proposed configurations', () => {
  const current = currentGoals();
  const proposed = {
    intervalGoal: {
      targetSeconds: 30,
      recurrence: 'daily' as const,
      alignment: { hour: 6 },
    },
  };

  const comparison = compareGoalConfigurations(current, proposed);

  assert.deepEqual(comparison, {
    current: currentGoals(),
    proposed: {
      intervalGoal: {
        targetSeconds: 30,
        recurrence: 'daily',
        alignment: { hour: 6 },
      },
    },
    changed: true,
  });

  current.intervalGoal.targetSeconds = 1;
  current.intervalGoal.alignment.hour = 0;
  current.overallTarget.targetSeconds = 1;
  proposed.intervalGoal.targetSeconds = 2;
  proposed.intervalGoal.alignment.hour = 12;

  assert.deepEqual(comparison, {
    current: currentGoals(),
    proposed: {
      intervalGoal: {
        targetSeconds: 30,
        recurrence: 'daily',
        alignment: { hour: 6 },
      },
    },
    changed: true,
  });
});

test('goal comparison ignores object insertion order for equivalent alignments', () => {
  const comparison = compareGoalConfigurations(
    {
      intervalGoal: {
        targetSeconds: 60,
        recurrence: 'yearly',
        alignment: { month: 2, day: 29 },
      },
    },
    {
      intervalGoal: {
        targetSeconds: 60,
        recurrence: 'yearly',
        alignment: { day: 29, month: 2 },
      },
    },
  );

  assert.equal(comparison.changed, false);
});

test('authoritative goal mutation reprojects 45 of 60 to uncapped 45 of 30 without disturbing running or unrelated state', () => {
  const selectedPath = {
    id: 'path-1',
    name: 'Read',
    visibility: 'private',
    capabilities: managerCapabilities,
    ...currentGoals(),
  };
  const state = {
    paths: [
      selectedPath,
      {
        id: 'path-2',
        name: 'Piano',
        visibility: 'followers',
        capabilities: managerCapabilities,
        intervalGoal: {
          targetSeconds: 600,
          recurrence: 'weekly' as const,
          alignment: { isoWeekday: 1 },
        },
      },
    ],
    selectedPath,
    timerStates: {
      'path-1': {
        running: true,
        accumulatedSeconds: 45,
        intervalProgress: { accumulatedSeconds: 45, targetSeconds: 60 },
        timer: { id: 'timer-1', startedAt: '2026-07-22T12:00:00Z' },
      },
      'path-2': {
        running: false,
        accumulatedSeconds: 120,
        intervalProgress: { accumulatedSeconds: 120, targetSeconds: 600 },
      },
    },
  };
  const untouchedPath = state.paths[1];
  const untouchedTimer = state.timerStates['path-2'];

  const next = applyGoalMutationResult(state, {
    path: {
      id: 'path-1',
      name: 'Read',
      visibility: 'private',
      capabilities: managerCapabilities,
      intervalGoal: {
        targetSeconds: 30,
        recurrence: 'daily',
        alignment: { hour: 6 },
      },
      overallTarget: { targetSeconds: 3_600 },
    },
    accumulatedSeconds: 45,
    intervalProgress: { accumulatedSeconds: 45, targetSeconds: 30 },
  });

  assert.deepEqual(next.paths[0]?.intervalGoal, {
    targetSeconds: 30,
    recurrence: 'daily',
    alignment: { hour: 6 },
  });
  assert.deepEqual(next.selectedPath?.intervalGoal, {
    targetSeconds: 30,
    recurrence: 'daily',
    alignment: { hour: 6 },
  });
  assert.deepEqual(next.selectedPath?.capabilities, managerCapabilities);
  assert.deepEqual(next.timerStates['path-1'], {
    running: true,
    accumulatedSeconds: 45,
    intervalProgress: { accumulatedSeconds: 45, targetSeconds: 30 },
    timer: { id: 'timer-1', startedAt: '2026-07-22T12:00:00Z' },
  });
  assert.strictEqual(next.paths[1], untouchedPath);
  assert.strictEqual(next.timerStates['path-2'], untouchedTimer);
  assert.equal(state.paths[0]?.intervalGoal?.targetSeconds, 60);
  assert.equal(state.selectedPath.intervalGoal.targetSeconds, 60);
  assert.equal(state.timerStates['path-1'].intervalProgress.targetSeconds, 60);
});

test('authoritative goal removal clears only its progress projection and retains activity and timer state', () => {
  const selectedPath = {
    id: 'path-1',
    name: 'Read',
    visibility: 'private',
    capabilities: managerCapabilities,
    ...currentGoals(),
  };
  const state = {
    paths: [selectedPath],
    selectedPath,
    timerStates: {
      'path-1': {
        running: true,
        accumulatedSeconds: 45,
        intervalProgress: { accumulatedSeconds: 45, targetSeconds: 60 },
        timer: { id: 'timer-1', startedAt: '2026-07-22T12:00:00Z' },
      },
    },
  };

  const next = applyGoalMutationResult(state, {
    path: {
      id: 'path-1',
      name: 'Read',
      visibility: 'private',
      capabilities: managerCapabilities,
      overallTarget: { targetSeconds: 3_600 },
    },
    accumulatedSeconds: 45,
  });

  assert.equal(next.paths[0]?.intervalGoal, undefined);
  assert.equal(next.selectedPath?.intervalGoal, undefined);
  assert.deepEqual(next.paths[0]?.overallTarget, { targetSeconds: 3_600 });
  assert.deepEqual(next.timerStates['path-1'], {
    running: true,
    accumulatedSeconds: 45,
    intervalProgress: undefined,
    timer: { id: 'timer-1', startedAt: '2026-07-22T12:00:00Z' },
  });
  assert.equal(state.paths[0]?.intervalGoal?.targetSeconds, 60);
  assert.equal(state.timerStates['path-1'].accumulatedSeconds, 45);
});
