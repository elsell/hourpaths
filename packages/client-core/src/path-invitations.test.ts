import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createPathInvitationAcceptOwner,
  createPathInvitationRejectOwner,
  createPathInvitationRecipientReviewOwner,
  createPathInvitationSendOwner,
  mergePendingInvitationPage,
  pathInvitationOutputData,
  pathInvitationFailureFromProblem,
  pathInvitationFailureMessageKey,
  type PathInvitation,
  type PathInvitationFailure,
  type PendingPathInvitation,
} from './index.js';
import {
  reviewPendingPathInvitationAcceptance,
  type PathInvitationAcceptBody,
} from './path-invitations.js';

const recipient = {
  userId: 'recipient-1',
  username: 'Reader.One',
  displayName: 'Reader One',
};

const pending: PathInvitation = {
  id: 'invitation-1',
  pathId: 'path-1',
  inviterUserId: 'creator',
  recipientUserId: recipient.userId,
  offeredRole: 'participant',
  createdAt: '2026-07-23T20:00:00Z',
};

const pendingProjection: PendingPathInvitation = {
  invitation: pending,
  pathName: 'Morning Reading',
  inviter: {
    userId: 'creator',
    username: 'Book.Owner',
    displayName: 'Book Owner',
  },
};

test('singleton invitation output extraction unwraps data and fails closed', () => {
  const output = { data: recipient };
  assert.equal(pathInvitationOutputData(output), recipient);
  for (const malformed of [
    undefined,
    null,
    [],
    {},
    { value: recipient },
    { data: undefined },
  ]) {
    assert.throws(
      () => pathInvitationOutputData(malformed),
      (error) => {
        assert.deepEqual(error, { kind: 'invalid_response' });
        return true;
      },
    );
  }
});

const warnedPendingProjection = {
  ...pendingProjection,
  warning: {
    pathVisibility: 'followers' as const,
    hasRetainedActivity: true,
  },
};

test('a new Path or username owns recipient review and supersedes stale completion', async () => {
  const owner = createPathInvitationRecipientReviewOwner();
  let resolveFirst!: (value: typeof recipient) => void;
  const firstResponse = new Promise<typeof recipient>((resolve) => { resolveFirst = resolve; });
  const first = owner.review('path-1', 'reader.one', async () => firstResponse);
  const second = owner.review('path-2', 'other.reader', async (pathId, username) => {
    assert.equal(pathId, 'path-2');
    assert.equal(username, 'other.reader');
    return { userId: 'recipient-2', username: 'Other.Reader', displayName: 'Other Reader' };
  });

  assert.deepEqual(await second, {
    kind: 'reviewed',
    review: {
      pathId: 'path-2',
      requestedUsername: 'other.reader',
      recipient: { userId: 'recipient-2', username: 'Other.Reader', displayName: 'Other Reader' },
    },
  });
  resolveFirst(recipient);
  assert.deepEqual(await first, { kind: 'superseded' });
});

test('review rejects a non-exact or malformed public identity projection', async () => {
  const owner = createPathInvitationRecipientReviewOwner();
  for (const malformed of [
    { ...recipient, username: 'different' },
    { ...recipient, userId: '' },
    { ...recipient, displayName: '' },
    { ...recipient, email: 'provider@example.com' },
  ]) {
    const result = await owner.review('path-1', 'reader.one', async () => malformed);
    assert.deepEqual(result, { kind: 'failed', failure: { kind: 'invalid_response' } });
  }
});

test('confirmed send uses reviewed canonical identity and exact role with a stable retry request', async () => {
  const reviewOwner = createPathInvitationRecipientReviewOwner();
  const reviewed = await reviewOwner.review('path-1', 'reader.one', async () => recipient);
  assert.equal(reviewed.kind, 'reviewed');
  if (reviewed.kind !== 'reviewed') return;

  const keys = ['send-invitation-key-0001', 'unused-send-key-0002'];
  const owner = createPathInvitationSendOwner(() => keys.shift()!);
  const calls: Array<{ pathId: string; key: string; body: unknown }> = [];
  const failure = { kind: 'network' } as const;
  const request = async (
    pathId: string,
    body: {
      username: string;
      expectedRecipientUserId: string;
      offeredRole: 'participant' | 'supporter';
    },
    key: string,
  ): Promise<PathInvitation> => {
    calls.push({ pathId, key, body });
    if (calls.length === 1) throw failure;
    return pending;
  };

  assert.deepEqual(
    await owner.submit(reviewed.review, 'participant', true, request),
    { kind: 'failed', failure },
  );
  assert.deepEqual(
    await owner.submit(reviewed.review, 'participant', true, request),
    { kind: 'sent', invitation: pending },
  );
  assert.deepEqual(calls.map(({ pathId, key }) => ({ pathId, key })), [
    { pathId: 'path-1', key: 'send-invitation-key-0001' },
    { pathId: 'path-1', key: 'send-invitation-key-0001' },
  ]);
  assert.strictEqual(calls[0]?.body, calls[1]?.body);
  assert.deepEqual(calls[0]?.body, {
    username: 'Reader.One',
    expectedRecipientUserId: 'recipient-1',
    offeredRole: 'participant',
  });
  assert.equal(Object.isFrozen(calls[0]?.body), true);
});

test('send cancellation makes no request and a changed role creates new intent', async () => {
  const reviewOwner = createPathInvitationRecipientReviewOwner();
  const reviewed = await reviewOwner.review('path-1', 'reader.one', async () => recipient);
  if (reviewed.kind !== 'reviewed') assert.fail('review failed');
  const keys = ['send-invitation-key-0001', 'send-invitation-key-0002'];
  const owner = createPathInvitationSendOwner(() => keys.shift()!);
  let calls = 0;
  assert.deepEqual(
    await owner.submit(reviewed.review, 'participant', false, async () => {
      calls += 1;
      return pending;
    }),
    { kind: 'cancelled' },
  );
  assert.equal(calls, 0);
  const supporter = { ...pending, offeredRole: 'supporter' as const };
  const result = await owner.submit(reviewed.review, 'supporter', true, async (_path, body, key) => {
    assert.deepEqual(body, {
      username: 'Reader.One',
      expectedRecipientUserId: 'recipient-1',
      offeredRole: 'supporter',
    });
    assert.equal(key, 'send-invitation-key-0001');
    return supporter;
  });
  assert.deepEqual(result, { kind: 'sent', invitation: supporter });
});

test('pending pages retain signed cursors exactly and merge without duplicate invitations', () => {
  const signed = 'eyJzbmFwc2hvdCI6IjIwMjYtMDctMjNUMjA6MDA6MDBaIn0.signature+/=';
  const first = mergePendingInvitationPage(
    { items: [], nextCursor: '' },
    {
      items: [
        pendingProjection,
        { ...pendingProjection, invitation: { ...pending, id: 'invitation-2' } },
      ],
      nextCursor: signed,
    },
    '',
  );
  assert.equal(first.nextCursor, signed);
  const updated = {
    ...pendingProjection,
    invitation: { ...pending, offeredRole: 'supporter' as const },
  };
  const second = mergePendingInvitationPage(
    first,
    {
      items: [
        updated,
        { ...pendingProjection, invitation: { ...pending, id: 'invitation-3' } },
        { ...pendingProjection, invitation: { ...pending, id: 'invitation-3' } },
      ],
      nextCursor: '',
    },
    signed,
  );
  assert.deepEqual(second.items, [
    updated,
    { ...pendingProjection, invitation: { ...pending, id: 'invitation-2' } },
    { ...pendingProjection, invitation: { ...pending, id: 'invitation-3' } },
  ]);
  assert.equal(second.nextCursor, '');
  assert.throws(
    () => mergePendingInvitationPage(first, { items: [], nextCursor: '' }, 'different.cursor'),
    /stale invitation cursor/,
  );
});

test('pending pages require Path and email-free inviter context', () => {
  for (const malformed of [
    { ...pendingProjection, pathName: '' },
    { ...pendingProjection, inviter: { ...pendingProjection.inviter, userId: 'different' } },
    { ...pendingProjection, inviter: { ...pendingProjection.inviter, displayName: '' } },
    { ...pendingProjection, inviter: { ...pendingProjection.inviter, email: 'owner@example.com' } },
    { ...pendingProjection, profileVisibility: 'public' },
  ]) {
    assert.throws(
      () => mergePendingInvitationPage(
        { items: [], nextCursor: '' },
        { items: [malformed as PendingPathInvitation], nextCursor: '' },
        '',
      ),
      /invalid pending invitation/,
    );
  }
});

test('pending pages strictly retain and freeze optional visibility warning context', () => {
  const source = {
    ...warnedPendingProjection,
    warning: { ...warnedPendingProjection.warning },
  };
  const merged = mergePendingInvitationPage(
    { items: [], nextCursor: '' },
    { items: [source], nextCursor: '' },
    '',
  );
  assert.deepEqual(merged.items[0]?.warning, {
    pathVisibility: 'followers',
    hasRetainedActivity: true,
  });
  assert.equal(Object.isFrozen(merged.items[0]?.warning), true);
  (source.warning as {
    pathVisibility: 'followers' | 'public';
    hasRetainedActivity: boolean;
  }).pathVisibility = 'public';
  source.warning.hasRetainedActivity = false;
  assert.deepEqual(merged.items[0]?.warning, {
    pathVisibility: 'followers',
    hasRetainedActivity: true,
  });

  for (const malformed of [
    { pathVisibility: 'private', hasRetainedActivity: true },
    { pathVisibility: 'followers' },
    { pathVisibility: 'public', hasRetainedActivity: 'yes' },
    { pathVisibility: 'public', hasRetainedActivity: false, detail: 'secret' },
    null,
  ]) {
    assert.throws(
      () => mergePendingInvitationPage(
        { items: [], nextCursor: '' },
        {
          items: [{ ...pendingProjection, warning: malformed } as PendingPathInvitation],
          nextCursor: '',
        },
        '',
      ),
      /invalid pending invitation/,
    );
  }
});

test('acceptance review produces an immutable warning intent or a ready intent', () => {
  const warningReview = reviewPendingPathInvitationAcceptance(warnedPendingProjection);
  assert.deepEqual(warningReview, {
    kind: 'confirmation-required',
    invitationId: 'invitation-1',
    warning: {
      pathVisibility: 'followers',
      hasRetainedActivity: true,
    },
  });
  assert.equal(Object.isFrozen(warningReview), true);
  assert.equal(
    warningReview.kind === 'confirmation-required' && Object.isFrozen(warningReview.warning),
    true,
  );
  assert.deepEqual(reviewPendingPathInvitationAcceptance(pendingProjection), {
    kind: 'ready',
    invitationId: 'invitation-1',
  });
});

test('accept retries each invitation with its own stable key and supersedes stale completion', async () => {
  const keys = [
    'accept-invitation-key-01',
    'accept-invitation-key-02',
    'accept-invitation-key-03',
  ];
  const owner = createPathInvitationAcceptOwner(() => keys.shift()!);
  const calls: Array<{ id: string; key: string }> = [];
  const warning = { kind: 'warning_required' } as const;
  const request = async (id: string, key: string): Promise<PathInvitation> => {
    calls.push({ id, key });
    if (calls.length === 1) throw warning;
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  };
  assert.deepEqual(await owner.accept('invitation-1', request), {
    kind: 'failed', failure: warning,
  });
  assert.equal((await owner.accept('invitation-1', request)).kind, 'accepted');
  assert.equal((await owner.accept('invitation-2', request)).kind, 'accepted');
  assert.deepEqual(calls, [
    { id: 'invitation-1', key: 'accept-invitation-key-01' },
    { id: 'invitation-1', key: 'accept-invitation-key-01' },
    { id: 'invitation-2', key: 'accept-invitation-key-02' },
  ]);

  let resolve!: (value: PathInvitation) => void;
  const pendingResponse = new Promise<PathInvitation>((done) => { resolve = done; });
  const stale = owner.accept('invitation-3', async () => pendingResponse);
  owner.cancel('invitation-3');
  resolve({ ...pending, id: 'invitation-3', acceptedAt: '2026-07-23T20:02:00Z' });
  assert.deepEqual(await stale, { kind: 'superseded' });
});

test('reject requires confirmation, retries each invitation with its own stable key, and validates the result', async () => {
  const keys = [
    'reject-invitation-key-01',
    'reject-invitation-key-02',
    'reject-invitation-key-03',
  ];
  const owner = createPathInvitationRejectOwner(() => keys.shift()!);
  const calls: Array<{ id: string; key: string }> = [];
  const request = async (id: string, key: string): Promise<unknown> => {
    calls.push({ id, key });
    if (calls.length === 1) throw { kind: 'network' } as const;
    return {
      invitationId: id,
      rejectedAt: '2026-07-23T20:01:00Z',
      unreadCount: id === 'invitation-1' ? 2 : 1,
    };
  };

  assert.deepEqual(await owner.submit('invitation-1', false, request), { kind: 'cancelled' });
  assert.equal(calls.length, 0);
  assert.deepEqual(await owner.submit('invitation-1', true, request), {
    kind: 'failed', failure: { kind: 'network' },
  });
  assert.deepEqual(await owner.submit('invitation-1', true, request), {
    kind: 'rejected',
    rejection: {
      invitationId: 'invitation-1',
      rejectedAt: '2026-07-23T20:01:00Z',
      unreadCount: 2,
    },
  });
  assert.equal((await owner.submit('invitation-2', true, request)).kind, 'rejected');
  assert.deepEqual(calls.map(({ id, key }) => ({ id, key })), [
    { id: 'invitation-1', key: 'reject-invitation-key-01' },
    { id: 'invitation-1', key: 'reject-invitation-key-01' },
    { id: 'invitation-2', key: 'reject-invitation-key-02' },
  ]);

  assert.deepEqual(await owner.submit('invitation-3', true, async () => ({
    invitationId: 'another-invitation',
    rejectedAt: '2026-07-23T20:01:00Z',
    unreadCount: 0,
  })), { kind: 'failed', failure: { kind: 'invalid_response' } });
});

test('reject cancellation supersedes stale completion and clears the retry intent', async () => {
  const keys = ['reject-invitation-key-01', 'reject-invitation-key-02'];
  const owner = createPathInvitationRejectOwner(() => keys.shift()!);
  let resolve!: (value: unknown) => void;
  const response = new Promise<unknown>((done) => { resolve = done; });
  const stale = owner.submit('invitation-1', true, async () => response);
  owner.cancel('invitation-1');
  resolve({
    invitationId: 'invitation-1',
    rejectedAt: '2026-07-23T20:01:00Z',
    unreadCount: 0,
  });
  assert.deepEqual(await stale, { kind: 'superseded' });

  const calls: string[] = [];
  assert.equal((await owner.submit('invitation-1', true, async (_id, key) => {
    calls.push(key);
    return {
      invitationId: 'invitation-1',
      rejectedAt: '2026-07-23T20:02:00Z',
      unreadCount: 0,
    };
  })).kind, 'rejected');
  assert.deepEqual(calls, ['reject-invitation-key-02']);
});

test('accept and reject preserve opaque failures distinctly while retaining stable retry keys', async () => {
  const accept = createPathInvitationAcceptOwner(() => 'accept-opaque-key-001');
  const reject = createPathInvitationRejectOwner(() => 'reject-opaque-key-001');
  const acceptKeys: string[] = [];
  const rejectKeys: string[] = [];

  assert.deepEqual(await accept.accept('invitation-1', async (_id, key) => {
    acceptKeys.push(key);
    throw { kind: 'opaque' } as const;
  }), { kind: 'failed', failure: { kind: 'opaque' } });
  assert.equal((await accept.accept('invitation-1', async (id, key) => {
    acceptKeys.push(key);
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  })).kind, 'accepted');

  assert.deepEqual(await reject.submit('invitation-1', true, async (_id, key) => {
    rejectKeys.push(key);
    throw { kind: 'opaque' } as const;
  }), { kind: 'failed', failure: { kind: 'opaque' } });
  assert.equal((await reject.submit('invitation-1', true, async (id, key) => {
    rejectKeys.push(key);
    return { invitationId: id, rejectedAt: '2026-07-23T20:01:00Z', unreadCount: 0 };
  })).kind, 'rejected');

  assert.deepEqual(acceptKeys, ['accept-opaque-key-001', 'accept-opaque-key-001']);
  assert.deepEqual(rejectKeys, ['reject-opaque-key-001', 'reject-opaque-key-001']);
});

test('reviewed warning acceptance cancels without a request and sends exact acknowledgement', async () => {
  const keys = ['accept-warning-key-01'];
  const owner = createPathInvitationAcceptOwner(() => keys.shift()!);
  const review = reviewPendingPathInvitationAcceptance(warnedPendingProjection);
  const calls: Array<{ id: string; key: string; body: PathInvitationAcceptBody | undefined }> = [];
  const request = async (
    id: string,
    key: string,
    body?: PathInvitationAcceptBody,
  ): Promise<PathInvitation> => {
    calls.push({ id, key, body });
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  };

  assert.deepEqual(await owner.submit(review, false, request), { kind: 'cancelled' });
  assert.equal(calls.length, 0);
  assert.deepEqual(await owner.submit(review, true, request), {
    kind: 'accepted',
    invitation: { ...pending, acceptedAt: '2026-07-23T20:01:00Z' },
  });
  assert.deepEqual(calls, [{
    id: 'invitation-1',
    key: 'accept-warning-key-01',
    body: {
      visibilityWarningAcknowledgement: { pathVisibility: 'followers' },
    },
  }]);
  assert.equal(Object.isFrozen(calls[0]?.body), true);
  assert.equal(Object.isFrozen(calls[0]?.body?.visibilityWarningAcknowledgement), true);
});

test('cancelling a failed warning acceptance clears its owned retry intent', async () => {
  const keys = ['accept-warning-key-01', 'accept-warning-key-02'];
  const owner = createPathInvitationAcceptOwner(() => keys.shift()!);
  const review = reviewPendingPathInvitationAcceptance(warnedPendingProjection);
  const seenKeys: string[] = [];
  assert.equal((await owner.submit(review, true, async (_id, key) => {
    seenKeys.push(key);
    throw { kind: 'network' };
  })).kind, 'failed');
  let cancelRequests = 0;
  assert.deepEqual(await owner.submit(review, false, async () => {
    cancelRequests += 1;
    return pending;
  }), { kind: 'cancelled' });
  assert.equal(cancelRequests, 0);
  assert.equal((await owner.submit(review, true, async (id, key) => {
    seenKeys.push(key);
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  })).kind, 'accepted');
  assert.deepEqual(seenKeys, ['accept-warning-key-01', 'accept-warning-key-02']);
});

test('ready acceptance sends no body and reviewed retries retain a frozen request intent', async () => {
  const keys = [
    'accept-ready-key-0001',
    'accept-warn-key-00002',
    'accept-changed-key-003',
  ];
  const owner = createPathInvitationAcceptOwner(() => keys.shift()!);
  const ready = reviewPendingPathInvitationAcceptance(pendingProjection);
  let readyBody: PathInvitationAcceptBody | undefined;
  assert.equal((await owner.submit(ready, false, async (id, _key, body) => {
    readyBody = body;
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  })).kind, 'accepted');
  assert.equal(readyBody, undefined);

  const source = {
    ...warnedPendingProjection,
    warning: { ...warnedPendingProjection.warning },
  };
  const review = reviewPendingPathInvitationAcceptance(source);
  const calls: Array<{ key: string; body: PathInvitationAcceptBody | undefined }> = [];
  const request = async (
    id: string,
    key: string,
    body?: PathInvitationAcceptBody,
  ): Promise<PathInvitation> => {
    calls.push({ key, body });
    if (calls.length === 1) throw { kind: 'network' };
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  };
  assert.equal((await owner.submit(review, true, request)).kind, 'failed');
  (source.warning as {
    pathVisibility: 'followers' | 'public';
    hasRetainedActivity: boolean;
  }).pathVisibility = 'public';
  assert.equal((await owner.submit(review, true, request)).kind, 'accepted');
  assert.equal(calls[0]?.key, 'accept-warn-key-00002');
  assert.equal(calls[1]?.key, 'accept-warn-key-00002');
  assert.strictEqual(calls[0]?.body, calls[1]?.body);
  assert.deepEqual(calls[1]?.body, {
    visibilityWarningAcknowledgement: { pathVisibility: 'followers' },
  });

  const changed = reviewPendingPathInvitationAcceptance({
    ...warnedPendingProjection,
    warning: {
      pathVisibility: 'public',
      hasRetainedActivity: false,
    },
  });
  assert.equal((await owner.submit(changed, true, async (id, key) => {
    assert.equal(key, 'accept-changed-key-003');
    return { ...pending, id, acceptedAt: '2026-07-23T20:01:00Z' };
  })).kind, 'accepted');
});

test('reviewed acceptance stays owner-safe across cancellation and stale warning failure', async () => {
  const owner = createPathInvitationAcceptOwner(() => 'accept-warning-key-01');
  const review = reviewPendingPathInvitationAcceptance(warnedPendingProjection);
  let reject!: (cause: unknown) => void;
  const response = new Promise<unknown>((_resolve, rejectPromise) => { reject = rejectPromise; });
  const stale = owner.submit(review, true, async () => response);
  owner.cancel('invitation-1');
  reject({
    kind: 'warning_required',
    title: 'Do not render',
    detail: 'private server detail',
  });
  assert.deepEqual(await stale, { kind: 'superseded' });

  assert.deepEqual(
    await owner.submit(review, true, async () => {
      throw {
        kind: 'warning_required',
        title: 'Do not render',
        detail: 'private server detail',
      };
    }),
    { kind: 'failed', failure: { kind: 'warning_required' } },
  );
});

test('problem classification keeps only trusted typed failure data', () => {
  assert.deepEqual(
    pathInvitationFailureFromProblem(404, {
      code: 'forbidden', title: 'User exists', detail: 'provider@example.com',
    }),
    { kind: 'opaque' },
  );
  assert.deepEqual(
    pathInvitationFailureFromProblem(409, {
      code: 'invitation_warning_required', detail: '<script>hostile</script>',
    }),
    { kind: 'warning_required' },
  );
  assert.deepEqual(
    pathInvitationFailureFromProblem(429, {
      code: 'rate_limited', title: 'Do not render this',
    }),
    { kind: 'http', status: 429, code: 'rate_limited' },
  );
  assert.deepEqual(
    pathInvitationFailureFromProblem(500, {
      code: 'unknown', detail: 'database secret',
    }),
    { kind: 'http', status: 500 },
  );
  const typed: PathInvitationFailure = { kind: 'network' };
  assert.deepEqual(typed, { kind: 'network' });
});

test('operation failures discard untrusted presentation fields', async () => {
  const owner = createPathInvitationAcceptOwner(() => 'accept-invitation-key-01');
  const result = await owner.accept('invitation-1', async () => {
    throw {
      kind: 'http',
      status: 500,
      code: 'internal_error',
      title: 'Database exploded',
      detail: 'provider@example.com',
    };
  });
  assert.deepEqual(result, {
    kind: 'failed',
    failure: { kind: 'http', status: 500, code: 'internal_error' },
  });
});

test('typed failures map only to localized invitation message keys', () => {
  assert.equal(pathInvitationFailureMessageKey({ kind: 'opaque' }), 'pathInvitation.unavailable');
  assert.equal(
    pathInvitationFailureMessageKey({ kind: 'warning_required' }),
    'pathInvitation.warningRequired',
  );
  assert.equal(pathInvitationFailureMessageKey({ kind: 'network' }), 'pathInvitation.retry');
  assert.equal(
    pathInvitationFailureMessageKey({ kind: 'http', status: 429, code: 'rate_limited' }),
    'pathInvitation.rateLimited',
  );
  assert.equal(
    pathInvitationFailureMessageKey({ kind: 'http', status: 503, code: 'authorization_pending' }),
    'pathInvitation.dependencyUnavailable',
  );
  assert.equal(
    pathInvitationFailureMessageKey({ kind: 'http', status: 409, code: 'conflict' }),
    'pathInvitation.failure',
  );
});
