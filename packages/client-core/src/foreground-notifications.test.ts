import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createForegroundNotificationCoordinator,
  type ForegroundNotificationContext,
  type ForegroundNotificationResolution,
} from './foreground-notifications.js';

const context = (
  targetKey: string | null = 'comments:event-1',
): ForegroundNotificationContext => ({
  sessionId: 'session-1',
  targetKey,
  userId: 'user-1',
});

test('a relevant receipt resolves and refreshes once across both native callbacks', async () => {
  let current = context();
  let resolveCount = 0;
  const effects: string[] = [];
  let releaseResolution: (() => void) | undefined;
  const coordinator = createForegroundNotificationCoordinator({
    current: () => current,
    resolve: async (): Promise<ForegroundNotificationResolution> => {
      resolveCount += 1;
      await new Promise<void>((resolve) => { releaseResolution = resolve; });
      return {
        presentation: 'actionable',
        recipientUserId: 'user-1',
        targetKey: 'comments:event-1',
      };
    },
    refreshHistory: async () => { effects.push('history'); },
    refreshRelevantTarget: async (targetKey) => { effects.push(`target:${targetKey}`); },
  });

  const handler = coordinator.request({
    notificationId: 'notification-1',
    sessionId: 'session-1',
    userId: 'user-1',
  });
  const listener = coordinator.request({
    notificationId: 'notification-1',
    sessionId: 'session-1',
    userId: 'user-1',
  });
  assert.equal(handler, listener);
  releaseResolution?.();

  const outcome = await handler;
  assert.deepEqual(
    {
      markRead: outcome.markRead,
      presentation: outcome.presentation,
      refreshHistory: outcome.refreshHistory,
      refreshRelevantTargetKey: outcome.refreshRelevantTargetKey,
    },
    {
      markRead: false,
      presentation: 'quiet',
      refreshHistory: true,
      refreshRelevantTargetKey: 'comments:event-1',
    },
  );
  await outcome.effects;
  assert.equal(resolveCount, 1);
  assert.deepEqual(effects, ['history', 'target:comments:event-1']);

  const cached = await coordinator.request({
    notificationId: 'notification-1',
    sessionId: 'session-1',
    userId: 'user-1',
  });
  await cached.effects;
  assert.equal(resolveCount, 1);
  assert.deepEqual(effects, ['history', 'target:comments:event-1']);

  current = context('path:path-1');
  coordinator.dispose();
});

test('unrelated actionable and informational receipts retain their presentation class', async () => {
  for (const presentation of ['actionable', 'informational'] as const) {
    const effects: string[] = [];
    const coordinator = createForegroundNotificationCoordinator({
      current: () => context('path:other'),
      resolve: async () => ({
        presentation,
        recipientUserId: 'user-1',
        targetKey: 'comments:event-1',
      }),
      refreshHistory: async () => { effects.push('history'); },
      refreshRelevantTarget: async () => { effects.push('target'); },
    });

    const outcome = await coordinator.request({
      notificationId: `notification-${presentation}`,
      sessionId: 'session-1',
      userId: 'user-1',
    });
    assert.equal(outcome.presentation, presentation);
    assert.equal(outcome.markRead, false);
    assert.equal(outcome.refreshRelevantTargetKey, null);
    await outcome.effects;
    assert.deepEqual(effects, ['history']);
  }
});

test('mismatched, stale, and unresolvable receipts fail quiet without read mutation', async () => {
  let current = context();
  let resolveCount = 0;
  const effects: string[] = [];
  const resolutions: Array<ForegroundNotificationResolution | null> = [
    {
      presentation: 'actionable',
      recipientUserId: 'user-1',
      targetKey: 'comments:event-1',
    },
    null,
  ];
  let releaseStale: (() => void) | undefined;
  const coordinator = createForegroundNotificationCoordinator({
    current: () => current,
    resolve: async () => {
      resolveCount += 1;
      if (resolveCount === 1) {
        await new Promise<void>((resolve) => { releaseStale = resolve; });
      }
      return resolutions.shift() ?? null;
    },
    refreshHistory: async () => { effects.push('history'); },
    refreshRelevantTarget: async () => { effects.push('target'); },
  });

  const mismatch = await coordinator.request({
    notificationId: 'notification-mismatch',
    sessionId: 'session-other',
    userId: 'user-1',
  });
  assert.deepEqual(
    [mismatch.presentation, mismatch.markRead, mismatch.refreshHistory],
    ['quiet', false, false],
  );
  assert.equal(resolveCount, 0);

  const userMismatch = await coordinator.request({
    notificationId: 'notification-user-mismatch',
    sessionId: 'session-1',
    userId: 'user-2',
  });
  assert.deepEqual(
    [userMismatch.presentation, userMismatch.markRead, userMismatch.refreshHistory],
    ['quiet', false, false],
  );
  assert.equal(resolveCount, 0);

  const stalePromise = coordinator.request({
    notificationId: 'notification-stale',
    sessionId: 'session-1',
    userId: 'user-1',
  });
  current = { ...context(), userId: 'user-2' };
  releaseStale?.();
  const stale = await stalePromise;
  assert.deepEqual(
    [stale.presentation, stale.markRead, stale.refreshHistory],
    ['quiet', false, false],
  );

  current = context();
  const unresolvable = await coordinator.request({
    notificationId: 'notification-unresolvable',
    sessionId: 'session-1',
    userId: 'user-1',
  });
  assert.deepEqual(
    [unresolvable.presentation, unresolvable.markRead, unresolvable.refreshHistory],
    ['quiet', false, true],
  );
  await unresolvable.effects;
  assert.deepEqual(effects, ['history']);

  const retriedUnresolvable = await coordinator.request({
    notificationId: 'notification-unresolvable',
    sessionId: 'session-1',
    userId: 'user-1',
  });
  await retriedUnresolvable.effects;
  assert.equal(resolveCount, 3);
  assert.deepEqual(effects, ['history', 'history']);
});

test('a resolution for another recipient fails quiet and is never cached', async () => {
  let resolveCount = 0;
  const effects: string[] = [];
  const coordinator = createForegroundNotificationCoordinator({
    current: () => context(),
    resolve: async () => {
      resolveCount += 1;
      return {
        presentation: 'actionable',
        recipientUserId: 'user-other',
        targetKey: 'comments:event-1',
      };
    },
    refreshHistory: async () => { effects.push('history'); },
    refreshRelevantTarget: async () => { effects.push('target'); },
  });
  const receipt = {
    notificationId: 'notification-cross-user',
    sessionId: 'session-1',
    userId: 'user-1',
  };

  for (let attempt = 0; attempt < 2; attempt += 1) {
    const outcome = await coordinator.request(receipt);
    assert.deepEqual(
      [outcome.presentation, outcome.markRead, outcome.refreshHistory],
      ['quiet', false, false],
    );
    await outcome.effects;
  }
  assert.equal(resolveCount, 2);
  assert.deepEqual(effects, []);
});

test('resolution and effect failures stay quiet and remain retryable', async () => {
  let resolveCount = 0;
  let historyCount = 0;
  const coordinator = createForegroundNotificationCoordinator({
    current: () => context('path:other'),
    resolve: async () => {
      resolveCount += 1;
      if (resolveCount === 1) throw new Error('temporary resolution failure');
      return {
        presentation: 'informational',
        recipientUserId: 'user-1',
        targetKey: 'comments:event-1',
      };
    },
    refreshHistory: async () => {
      historyCount += 1;
      if (historyCount === 2) throw new Error('temporary history failure');
    },
    refreshRelevantTarget: async () => {},
  });
  const receipt = {
    notificationId: 'notification-retry',
    sessionId: 'session-1',
    userId: 'user-1',
  };

  const resolutionFailure = await coordinator.request(receipt);
  assert.equal(resolutionFailure.presentation, 'quiet');
  assert.equal(resolutionFailure.markRead, false);
  await resolutionFailure.effects;
  assert.equal(resolutionFailure.refreshHistory, true);
  assert.equal(historyCount, 1);

  const effectFailure = await coordinator.request(receipt);
  assert.equal(effectFailure.presentation, 'informational');
  await assert.rejects(effectFailure.effects, /temporary history failure/);

  const recovered = await coordinator.request(receipt);
  await recovered.effects;
  assert.equal(recovered.presentation, 'informational');
  assert.equal(resolveCount, 3);
  assert.equal(historyCount, 3);
});
