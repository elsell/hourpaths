import assert from 'node:assert/strict';
import test from 'node:test';
import {
  NUDGE_AUDIENCES,
  NUDGE_PRESETS,
  createNudgeAudienceOperationOwner,
  createNudgeChannelOperationOwner,
  createNudgeSendOperationOwner,
  nudgeAudiencePreferenceFromAPI,
  nudgeEligibilityFromAPI,
  nudgeChannelPreferenceFromAPI,
  nudgeReceiptFromAPI,
  reviewNudgeAudienceChange,
  reviewNudgeChannelChange,
  reviewNudgeSend,
} from './nudges.js';

test('the public nudge catalog is the exact approved preset and audience set', () => {
  assert.deepEqual(NUDGE_PRESETS, [
    'you_have_got_this',
    'lets_go',
    'little_progress_counts',
    'keep_it_going',
    'time_to_work',
  ]);
  assert.deepEqual(NUDGE_AUDIENCES, ['nobody', 'path_members', 'followers', 'everyone']);
  assert.equal(Object.isFrozen(NUDGE_PRESETS), true);
  assert.equal(Object.isFrozen(NUDGE_AUDIENCES), true);
});

test('eligibility admits only the authoritative bounded reason catalog', () => {
  assert.deepEqual(nudgeEligibilityFromAPI({ eligible: true, pathId: 'path-1', recipientUserId: 'person-2' }), {
    eligible: true,
    pathId: 'path-1',
    recipientUserId: 'person-2',
  });
  assert.deepEqual(
    nudgeEligibilityFromAPI({ eligible: false, pathId: 'path-1', reason: 'goal_complete', recipientUserId: 'person-2' }),
    { eligible: false, pathId: 'path-1', reason: 'goal_complete', recipientUserId: 'person-2' },
  );
  assert.deepEqual(
    nudgeEligibilityFromAPI({ eligible: false, pathId: 'path-1', reason: 'rate_limited', recipientUserId: 'person-2' }),
    { eligible: false, pathId: 'path-1', reason: 'rate_limited', recipientUserId: 'person-2' },
  );
  for (const invalid of [
    { eligible: true, pathId: 'path-1', reason: 'rate_limited', recipientUserId: 'person-2' },
    { eligible: false, pathId: 'path-1', recipientUserId: 'person-2' },
    { eligible: false, pathId: 'path-1', reason: 'already_nudged', recipientUserId: 'person-2' },
    { eligible: false, pathId: 'path-1', reason: 'audience_disallows', recipientUserId: 'person-2' },
    { eligible: false, pathId: 'path-1', reason: 'blocked', recipientUserId: 'person-2' },
    { eligible: true, pathId: 'path-1', recipientUserId: 'person-2', customText: 'hello' },
    { eligible: 1, pathId: 'path-1', recipientUserId: 'person-2' },
  ]) assert.throws(() => nudgeEligibilityFromAPI(invalid), /invalid nudge eligibility/);
});

test('audience preference is exact, revisioned, and rejects hidden extension fields', () => {
  assert.deepEqual(
    nudgeAudiencePreferenceFromAPI({ audience: 'path_members', pathId: 'path-1', revision: 0, userId: 'person-1' }),
    { audience: 'path_members', pathId: 'path-1', revision: 0, userId: 'person-1' },
  );
  assert.deepEqual(
    nudgeAudiencePreferenceFromAPI({ audience: 'path_members', pathId: 'path-1', revision: 1, userId: 'person-1' }),
    { audience: 'path_members', pathId: 'path-1', revision: 1, userId: 'person-1' },
  );
  for (const invalid of [
    { audience: 'friends', pathId: 'path-1', revision: 1, userId: 'person-1' },
    { audience: 'everyone', pathId: 'path-1', revision: 0, userId: 'person-1' },
    { audience: 'nobody', pathId: 'path-1', revision: 1.5, userId: 'person-1' },
    { audience: 'followers', pathId: 'path-1', revision: 2, userId: 'person-1', updatedAt: '2026-08-05T12:00:00Z' },
  ]) assert.throws(() => nudgeAudiencePreferenceFromAPI(invalid), /invalid nudge audience preference/);
});

test('send reviews and receipts preserve only a stable preset content kind', () => {
  assert.deepEqual(reviewNudgeSend('path-1', 'person-2', 'lets_go'), {
    body: { content: { kind: 'preset', preset: 'lets_go' } },
    pathId: 'path-1',
    recipientUserId: 'person-2',
    signature: 'path-1\u0000person-2\u0000lets_go',
  });
  const receipt = {
    content: { kind: 'preset', preset: 'lets_go' },
    id: 'nudge-1',
    pathId: 'path-1',
    recipientUserId: 'person-2',
    senderUserId: 'person-1',
    sentAt: '2026-08-05T12:00:00Z',
  };
  assert.deepEqual(nudgeReceiptFromAPI(receipt), receipt);
  for (const invalid of [
    { ...receipt, content: { kind: 'custom', text: 'Keep going' } },
    { ...receipt, content: { kind: 'preset', preset: 'unknown' } },
    { ...receipt, content: { kind: 'preset', preset: 'lets_go', text: 'custom' } },
    { ...receipt, message: 'custom' },
    { ...receipt, recipientUserId: ' path-user ' },
    { ...receipt, sentAt: 'tomorrow' },
  ]) assert.throws(() => nudgeReceiptFromAPI(invalid), /invalid nudge receipt/);
  assert.throws(() => reviewNudgeSend('path-1', 'person-2', 'custom' as 'lets_go'), /invalid nudge send review/);
});

test('send ownership permits one request, reuses its key for retry, and ignores stale completion', async () => {
  const keys = ['nudge-send-key-0001', 'nudge-send-key-0002'];
  const owner = createNudgeSendOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const review = reviewNudgeSend('path-1', 'person-2', 'keep_it_going');
  const seen: Array<{ key: string; pathId: string; recipientUserId: string }> = [];

  const failure = await owner.submit(review, async (pathId, recipientUserId, _body, key) => {
    seen.push({ key, pathId, recipientUserId });
    throw new Error('offline');
  });
  assert.equal(failure.kind, 'failed');
  const applied = await owner.submit(review, async (pathId, recipientUserId, body, key) => {
    seen.push({ key, pathId, recipientUserId });
    return {
      content: body.content,
      id: 'nudge-1',
      pathId: 'path-1',
      recipientUserId: 'person-2',
      senderUserId: 'person-1',
      sentAt: '2026-08-05T12:00:00Z',
    };
  });
  assert.equal(applied.kind, 'applied');
  assert.deepEqual(seen, [
    { key: 'nudge-send-key-0001', pathId: 'path-1', recipientUserId: 'person-2' },
    { key: 'nudge-send-key-0001', pathId: 'path-1', recipientUserId: 'person-2' },
  ]);

  let resolve!: (value: unknown) => void;
  const pending = owner.submit(reviewNudgeSend('path-1', 'person-3', 'lets_go'), () => new Promise((next) => { resolve = next; }));
  const busy = await owner.submit(review, async () => { throw new Error('must not run'); });
  assert.deepEqual(busy, { kind: 'busy' });
  owner.cancel();
  resolve({
    content: { kind: 'preset', preset: 'lets_go' },
    id: 'nudge-stale',
    pathId: 'path-1',
    recipientUserId: 'person-3',
    senderUserId: 'person-1',
    sentAt: '2026-08-05T12:01:00Z',
  });
  assert.deepEqual(await pending, { kind: 'superseded' });
});

test('audience changes send expected revision, serialize mutations, and retain a retry key', async () => {
  const keys = ['nudge-audience-key-0001', 'nudge-audience-key-0002'];
  const owner = createNudgeAudienceOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const review = reviewNudgeAudienceChange({ audience: 'path_members', pathId: 'path-1', revision: 4, userId: 'person-1' }, 'followers');
  assert.deepEqual(review, {
    body: { audience: 'followers', expectedRevision: 4 },
    changed: true,
    pathId: 'path-1',
    signature: 'path-1\u0000person-1\u00004\u0000followers',
    userId: 'person-1',
  });
  const seen: Array<{ body: unknown; key: string }> = [];
  const failed = await owner.submit('path-1', review, async (_pathId, body, key) => {
    seen.push({ body, key });
    throw new Error('offline');
  });
  assert.equal(failed.kind, 'failed');
  const applied = await owner.submit('path-1', review, async (_pathId, body, key) => {
    seen.push({ body, key });
    return { audience: body.audience, pathId: 'path-1', revision: 5, userId: 'person-1' };
  });
  assert.deepEqual(applied, { kind: 'applied', preference: { audience: 'followers', pathId: 'path-1', revision: 5, userId: 'person-1' } });
  assert.deepEqual(seen, [
    { body: { audience: 'followers', expectedRevision: 4 }, key: 'nudge-audience-key-0001' },
    { body: { audience: 'followers', expectedRevision: 4 }, key: 'nudge-audience-key-0001' },
  ]);
  assert.throws(
    () => reviewNudgeAudienceChange({ audience: 'path_members', pathId: 'path-1', revision: 4, userId: 'person-1' }, 'invalid' as 'nobody'),
    /invalid nudge audience review/,
  );
});

test('audience operation rejects unchanged and stale authoritative responses', async () => {
  const owner = createNudgeAudienceOperationOwner(() => 'nudge-audience-key-0001');
  const unchanged = reviewNudgeAudienceChange({ audience: 'everyone', pathId: 'path-1', revision: 2, userId: 'person-1' }, 'everyone');
  assert.equal(unchanged.changed, false);
  await assert.rejects(() => owner.submit('path-1', unchanged, async () => ({ audience: 'everyone', pathId: 'path-1', revision: 3, userId: 'person-1' })), /unchanged/);

  const changed = reviewNudgeAudienceChange({ audience: 'everyone', pathId: 'path-1', revision: 2, userId: 'person-1' }, 'nobody');
  const result = await owner.submit('path-1', changed, async () => ({ audience: 'nobody', pathId: 'path-1', revision: 2, userId: 'person-1' }));
  assert.equal(result.kind, 'failed');
  assert.match(String(result.kind === 'failed' && result.cause), /invalid nudge audience result/);
});

test('audience ownership permits one request and cancellation supersedes stale completion', async () => {
  const owner = createNudgeAudienceOperationOwner(() => 'nudge-audience-key-0001');
  const review = reviewNudgeAudienceChange(
    { audience: 'path_members', pathId: 'path-1', revision: 2, userId: 'person-1' },
    'everyone',
  );
  let resolve!: (value: unknown) => void;
  const pending = owner.submit('path-1', review, () => new Promise((next) => { resolve = next; }));
  assert.deepEqual(
    await owner.submit('path-1', review, async () => { throw new Error('must not run'); }),
    { kind: 'busy' },
  );
  owner.cancel();
  resolve({ audience: 'everyone', pathId: 'path-1', revision: 3, userId: 'person-1' });
  assert.deepEqual(await pending, { kind: 'superseded' });
});

test('nudge notification channel preference is exact and revision zero is only the enabled default', () => {
  assert.deepEqual(
    nudgeChannelPreferenceFromAPI({ channel: 'nudges', enabled: true, revision: 0 }),
    { channel: 'nudges', enabled: true, revision: 0 },
  );
  assert.deepEqual(
    nudgeChannelPreferenceFromAPI({ channel: 'nudges', enabled: false, revision: 3 }),
    { channel: 'nudges', enabled: false, revision: 3 },
  );
  for (const invalid of [
    { channel: 'nudges', enabled: false, revision: 0 },
    { channel: 'nudges', enabled: true, revision: -1 },
    { channel: 'nudges', enabled: true, revision: 1.5 },
    { channel: 'following', enabled: true, revision: 1 },
    { channel: 'nudges', enabled: 1, revision: 1 },
    { channel: 'nudges', enabled: true, revision: 1, updatedAt: '2026-08-05T12:00:00Z' },
  ]) assert.throws(() => nudgeChannelPreferenceFromAPI(invalid), /invalid nudge notification channel preference/);
});

test('nudge notification channel review is revisioned and records unchanged intent', () => {
  assert.deepEqual(
    reviewNudgeChannelChange({ channel: 'nudges', enabled: true, revision: 0 }, false),
    {
      body: { enabled: false, expectedRevision: 0 },
      changed: true,
      signature: 'nudges\u00000\u0000false',
    },
  );
  assert.deepEqual(
    reviewNudgeChannelChange({ channel: 'nudges', enabled: false, revision: 2 }, false),
    {
      body: { enabled: false, expectedRevision: 2 },
      changed: false,
      signature: 'nudges\u00002\u0000false',
    },
  );
});

test('nudge notification channel changes serialize and retain one key for a stable retry', async () => {
  const keys = ['nudge-channel-key-0001', 'nudge-channel-key-0002'];
  const owner = createNudgeChannelOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const review = reviewNudgeChannelChange({ channel: 'nudges', enabled: true, revision: 0 }, false);
  const seen: Array<{ body: unknown; key: string }> = [];
  const failed = await owner.submit(review, async (body, key) => {
    seen.push({ body, key });
    throw new Error('offline');
  });
  assert.equal(failed.kind, 'failed');
  const applied = await owner.submit(review, async (body, key) => {
    seen.push({ body, key });
    return { channel: 'nudges', enabled: false, revision: 1 };
  });
  assert.deepEqual(applied, {
    kind: 'applied',
    preference: { channel: 'nudges', enabled: false, revision: 1 },
  });
  assert.deepEqual(seen, [
    { body: { enabled: false, expectedRevision: 0 }, key: 'nudge-channel-key-0001' },
    { body: { enabled: false, expectedRevision: 0 }, key: 'nudge-channel-key-0001' },
  ]);
});

test('nudge notification channel ownership rejects unchanged and stale results', async () => {
  const owner = createNudgeChannelOperationOwner(() => 'nudge-channel-key-0001');
  const unchanged = reviewNudgeChannelChange({ channel: 'nudges', enabled: true, revision: 2 }, true);
  await assert.rejects(
    () => owner.submit(unchanged, async () => ({ channel: 'nudges', enabled: true, revision: 3 })),
    /unchanged/,
  );

  const changed = reviewNudgeChannelChange({ channel: 'nudges', enabled: true, revision: 2 }, false);
  for (const response of [
    { channel: 'nudges', enabled: false, revision: 2 },
    { channel: 'nudges', enabled: false, revision: 4 },
    { channel: 'nudges', enabled: true, revision: 3 },
  ]) {
    const result = await owner.submit(changed, async () => response);
    assert.equal(result.kind, 'failed');
    assert.match(String(result.kind === 'failed' && result.cause), /invalid nudge notification channel result/);
  }
});

test('nudge notification channel cancellation supersedes completion and clears its retry', async () => {
  const keys = ['nudge-channel-key-0001', 'nudge-channel-key-0002'];
  const owner = createNudgeChannelOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const review = reviewNudgeChannelChange({ channel: 'nudges', enabled: true, revision: 3 }, false);
  let resolve!: (value: unknown) => void;
  const pending = owner.submit(review, () => new Promise((next) => { resolve = next; }));
  assert.deepEqual(
    await owner.submit(review, async () => { throw new Error('must not run'); }),
    { kind: 'busy' },
  );
  owner.cancel();
  resolve({ channel: 'nudges', enabled: false, revision: 4 });
  assert.deepEqual(await pending, { kind: 'superseded' });

  const retry = await owner.submit(review, async (_body, key) => {
    assert.equal(key, 'nudge-channel-key-0002');
    return { channel: 'nudges', enabled: false, revision: 4 };
  });
  assert.equal(retry.kind, 'applied');
});
