import assert from 'node:assert/strict';
import test from 'node:test';
import { createPathSubmissionOwner, type CreatedPath, type PathCreateBody } from './path-submission.js';

const creatorCapabilities = {
  trackTime: true,
  renamePath: true,
  inviteMembers: true,
  manageMembers: true,
  manageGoals: true,
  manageLifecycle: true,
  manageVisibility: true,
  transferOwnership: true,
  leavePath: false,
};

const deferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (cause: unknown) => void;
  const promise = new Promise<T>((accept, fail) => { resolve = accept; reject = fail; });
  return { promise, resolve, reject };
};

const createdPath = (body: PathCreateBody): CreatedPath => ({
  id: 'path-1',
  name: body.name,
  visibility: body.visibility ?? 'private',
  capabilities: creatorCapabilities,
  ...(body.intervalGoal ? {
    intervalGoal: {
      ...body.intervalGoal,
      alignment: body.intervalGoal.alignment ?? { minute: 0 },
    },
  } : {}),
  ...(body.overallTarget ? { overallTarget: body.overallTarget } : {}),
});

test('a failed create retries the exact canonical optional-goals body with the same idempotency key', async () => {
  const owner = createPathSubmissionOwner(() => '0123456789abcdef');
  const requests: Array<[PathCreateBody, string]> = [];
  const failure = new Error('offline');
  const request = async (body: PathCreateBody, key: string) => {
    requests.push([body, key]);
    if (requests.length === 1) throw failure;
    return createdPath(body);
  };
  const draft = {
    name: '  Piano  ',
    visibility: 'followers' as const,
    intervalGoal: {
      targetSeconds: 1800,
      recurrence: 'yearly' as const,
      alignment: { month: 2, day: 29 },
    },
    overallTarget: { targetSeconds: 360000 },
  };

  assert.deepEqual(await owner.submit(draft, request), { kind: 'failed', cause: failure });
  assert.deepEqual(await owner.submit({ ...draft, name: 'Piano' }, request), {
    kind: 'created', path: {
      id: 'path-1',
      name: 'Piano',
      visibility: 'followers',
      capabilities: creatorCapabilities,
      intervalGoal: draft.intervalGoal,
      overallTarget: draft.overallTarget,
    },
  });
  assert.deepEqual(requests, [
    [{ ...draft, name: 'Piano' }, '0123456789abcdef'],
    [{ ...draft, name: 'Piano' }, '0123456789abcdef'],
  ]);
});

test('changing any goal field after a failure starts a distinct idempotent operation', async () => {
  const keys = ['0123456789abcdef', 'fedcba9876543210'];
  const owner = createPathSubmissionOwner(() => keys.shift()!);
  const requests: Array<[PathCreateBody, string]> = [];
  const request = async (body: PathCreateBody, key: string) => {
    requests.push([body, key]);
    throw new Error('offline');
  };

  await owner.submit({ name: 'Read', overallTarget: { targetSeconds: 3600 } }, request);
  await owner.submit({ name: 'Read', overallTarget: { targetSeconds: 7200 } }, request);

  assert.deepEqual(requests.map(([body, key]) => [body.overallTarget?.targetSeconds, key]), [
    [3600, '0123456789abcdef'],
    [7200, 'fedcba9876543210'],
  ]);
});

test('changing visibility after a failure starts a distinct idempotent operation', async () => {
  const keys = ['0123456789abcdef', 'fedcba9876543210'];
  const owner = createPathSubmissionOwner(() => keys.shift()!);
  const requests: Array<[PathCreateBody, string]> = [];
  const request = async (body: PathCreateBody, key: string) => {
    requests.push([body, key]);
    throw new Error('offline');
  };

  await owner.submit({ name: 'Read', visibility: 'private' }, request);
  await owner.submit({ name: 'Read', visibility: 'followers' }, request);

  assert.deepEqual(requests, [
    [{ name: 'Read', visibility: 'private' }, '0123456789abcdef'],
    [{ name: 'Read', visibility: 'followers' }, 'fedcba9876543210'],
  ]);
});

test('caller and request mutations cannot alter the body retained for retry', async () => {
  const owner = createPathSubmissionOwner(() => '0123456789abcdef');
  const draft = {
    name: 'Read',
    intervalGoal: { targetSeconds: 900, recurrence: 'daily' as const, alignment: { hour: 6 } },
  };
  let calls = 0;
  const request = async (body: PathCreateBody) => {
    calls += 1;
    if (calls === 1) {
      body.intervalGoal!.targetSeconds = 1;
      throw new Error('offline');
    }
    return createdPath(body);
  };

  await owner.submit(draft, request);
  draft.intervalGoal.targetSeconds = 2;
  const result = await owner.submit({
    name: 'Read',
    intervalGoal: { targetSeconds: 900, recurrence: 'daily', alignment: { hour: 6 } },
  }, request);

  assert.equal(result.kind, 'created');
  if (result.kind === 'created') assert.equal(result.path.intervalGoal?.targetSeconds, 900);
});

test('omitted zero-valued hourly and daily defaults share retry identity with their explicit forms', async () => {
  for (const [recurrence, alignment] of [
    ['hourly', { minute: 0 }],
    ['daily', { hour: 0 }],
  ] as const) {
    const owner = createPathSubmissionOwner(() => '0123456789abcdef');
    const requests: Array<[PathCreateBody, string]> = [];
    const request = async (body: PathCreateBody, key: string) => {
      requests.push([body, key]);
      throw new Error('offline');
    };

    await owner.submit({ name: 'Read', intervalGoal: { targetSeconds: 60, recurrence } }, request);
    await owner.submit({ name: 'Read', intervalGoal: { targetSeconds: 60, recurrence, alignment } }, request);

    assert.deepEqual(requests, [
      [{ name: 'Read', intervalGoal: { targetSeconds: 60, recurrence, alignment } }, '0123456789abcdef'],
      [{ name: 'Read', intervalGoal: { targetSeconds: 60, recurrence, alignment } }, '0123456789abcdef'],
    ]);
  }
});

test('profile and nonzero defaults remain distinct from explicit alignment', async () => {
  for (const [recurrence, alignment] of [
    ['weekly', { isoWeekday: 1 }],
    ['monthly', { day: 1 }],
    ['yearly', { month: 1, day: 1 }],
  ] as const) {
    const keys = ['0123456789abcdef', 'fedcba9876543210'];
    const owner = createPathSubmissionOwner(() => keys.shift()!);
    const requests: Array<[PathCreateBody, string]> = [];
    const request = async (body: PathCreateBody, key: string) => {
      requests.push([body, key]);
      throw new Error('offline');
    };

    await owner.submit({ name: 'Read', intervalGoal: { targetSeconds: 60, recurrence } }, request);
    await owner.submit({ name: 'Read', intervalGoal: { targetSeconds: 60, recurrence, alignment } }, request);

    assert.equal(requests[0]![0].intervalGoal?.alignment, undefined);
    assert.deepEqual(requests[1]![0].intervalGoal?.alignment, alignment);
    assert.deepEqual(requests.map(([, key]) => key), ['0123456789abcdef', 'fedcba9876543210']);
  }
});

test('malformed goal combinations fail closed without allocating a key or calling the API', async () => {
  const malformed: PathCreateBody[] = [
    { name: 'Read', intervalGoal: { targetSeconds: 1, recurrence: 'hourly', alignment: {} } },
    { name: 'Read', intervalGoal: { targetSeconds: 1, recurrence: 'daily', alignment: { hour: 6, minute: 1 } } },
    { name: 'Read', intervalGoal: { targetSeconds: 1, recurrence: 'weekly', alignment: { isoWeekday: 1, day: 1 } } },
    { name: 'Read', intervalGoal: { targetSeconds: 1, recurrence: 'monthly', alignment: { day: 1, hour: 1 } } },
    { name: 'Read', intervalGoal: { targetSeconds: 1, recurrence: 'yearly', alignment: { month: 2, day: 30 } } },
    { name: 'Read', intervalGoal: { targetSeconds: 0, recurrence: 'hourly' } },
    { name: 'Read', overallTarget: { targetSeconds: -1 } },
  ];
  let keys = 0;
  let requests = 0;
  const owner = createPathSubmissionOwner(() => {
    keys += 1;
    return '0123456789abcdef';
  });

  for (const draft of malformed) {
    assert.deepEqual(await owner.submit(draft, async () => {
      requests += 1;
      throw new Error('must not run');
    }), { kind: 'invalid_goal' });
  }
  assert.equal(keys, 0);
  assert.equal(requests, 0);
});

test('a newer create supersedes stale completion and receives a distinct request key', async () => {
  const keys = ['0123456789abcdef', 'fedcba9876543210'];
  const owner = createPathSubmissionOwner(() => keys.shift()!);
  const first = deferred<CreatedPath>();
  const second = deferred<CreatedPath>();

  const firstResult = owner.submit({ name: 'Piano' }, () => first.promise);
  const secondResult = owner.submit({ name: 'Reading' }, () => second.promise);
  second.resolve({ id: 'path-2', name: 'Reading', visibility: 'public', capabilities: creatorCapabilities });
  assert.equal((await secondResult).kind, 'created');
  first.resolve({ id: 'path-1', name: 'Piano', visibility: 'private', capabilities: creatorCapabilities });
  assert.deepEqual(await firstResult, { kind: 'superseded' });
});

test('cancel supersedes in-flight work and blank names never call the API', async () => {
  const owner = createPathSubmissionOwner(() => '0123456789abcdef');
  let calls = 0;
  assert.deepEqual(await owner.submit({ name: '   ' }, async () => {
    calls += 1;
    throw new Error('must not run');
  }), { kind: 'invalid_name' });
  const pending = deferred<CreatedPath>();
  const result = owner.submit({ name: 'Read' }, () => pending.promise);
  owner.cancel();
  pending.resolve({ id: 'path-1', name: 'Read', visibility: 'followers', capabilities: creatorCapabilities });
  assert.deepEqual(await result, { kind: 'superseded' });
  assert.equal(calls, 0);
});

test('idempotency keys must be 16-128 printable ASCII characters', async () => {
  for (const key of ['too-short', `0123456789abcde\n`, 'x'.repeat(129)]) {
    const owner = createPathSubmissionOwner(() => key);
    await assert.rejects(owner.submit({ name: 'Read' }, async () => ({
      id: 'path',
      name: 'Read',
      visibility: 'private',
      capabilities: creatorCapabilities,
    })), /idempotency/i);
  }
});
