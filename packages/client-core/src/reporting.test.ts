import assert from 'node:assert/strict';
import test from 'node:test';
import { createReportSubmissionOwner } from './reporting';

test('ambiguous report retry preserves identity and account replacement fences its acknowledgment', async () => {
  let sequence = 0;
  const owner = createReportSubmissionOwner(() => `report-key-${++sequence}`);
  const draft = { target: { kind: 'profile', id: 'subject' }, reason: 'spam_or_scam', explanation: ' context ' } as const;
  const keys: string[] = [];
  const fail = async (_draft: unknown, key: string) => { keys.push(key); throw new Error('connection lost'); };
  await owner.submit(draft, fail);
  await owner.submit(draft, fail);
  assert.deepEqual(keys, ['report-key-1', 'report-key-1']);
  let resolve!: (value: { id: string }) => void;
  const pending = owner.submit(draft, () => new Promise(done => { resolve = done; }));
  owner.cancel();
  const next = await owner.submit(draft, async (_draft, key) => { keys.push(key); return { id: 'new-report' }; });
  resolve({ id: 'old-report' });
  assert.equal((await pending).kind, 'superseded');
  assert.equal(next.kind, 'submitted');
  assert.equal(keys.at(-1), 'report-key-2');
});
