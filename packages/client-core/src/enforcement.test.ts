import test from 'node:test';
import assert from 'node:assert/strict';
import { appealAvailable, createAppealSubmissionOwner, type EnforcementAppeal, type EnforcementNotice } from './enforcement';

test('appeal retry retains identity, while account replacement discards late receipt', async () => {
  let sequence = 0;
  const owner = createAppealSubmissionOwner(() => `appeal-key-${++sequence}`);
  const keys: string[] = [];
  const request = async (_id: string, explanation: string, key: string) => {
    keys.push(key);
    if (keys.length === 1) throw new Error('response lost');
    return { id: key, explanation, submittedAt: '2026-10-09T12:00:00Z' };
  };
  assert.equal((await owner.submit('notice', ' context ', request)).kind, 'failed');
  assert.equal((await owner.submit('notice', 'context', request)).kind, 'submitted');
  assert.deepEqual(keys, ['appeal-key-1', 'appeal-key-1']);
  let resolve!: (appeal: EnforcementAppeal) => void;
  const pending = owner.submit('notice-two', '', async () => new Promise<EnforcementAppeal>(r => { resolve = r; }));
  assert.equal((await owner.submit('notice-two', '', request)).kind, 'busy');
  owner.cancel();
  resolve({ id: 'appeal-key-2', explanation: '', submittedAt: '2026-10-09T12:00:00Z' });
  assert.equal((await pending).kind, 'superseded');
});
test('appeal closes at its absolute deadline and once submitted', () => {
  const notice: EnforcementNotice = { id: 'n', action: 'warning', policyReason: 'Policy', issuedAt: '2026-10-09T12:00:00Z', appealDeadline: '2026-11-08T12:00:00Z' };
  const deadline = Date.parse(notice.appealDeadline);
  assert.equal(appealAvailable(notice, deadline - 1), true);
  assert.equal(appealAvailable(notice, deadline), false);
  assert.equal(appealAvailable({ ...notice, appeal: { id: 'a', explanation: '', submittedAt: notice.issuedAt } }, deadline - 1), false);
});

test('API notice mapper rejects incomplete final decisions and drops private fields', async () => {
  const { enforcementNoticeFromAPI } = await import('./adapters/api-enforcement');
  const source = { id: 'n', action: 'warning', policyReason: 'Policy', issuedAt: '2026-10-09T12:00:00Z', appealDeadline: '2026-11-08T12:00:00Z', reporter: 'private', appeal: { id: 'a', explanation: 'Context', submittedAt: '2026-10-09T13:00:00Z', reviewer: 'private' } };
  const result = enforcementNoticeFromAPI(source);
  assert.equal('reporter' in result, false);
  assert.equal('reviewer' in result.appeal!, false);
  assert.throws(() => enforcementNoticeFromAPI({ ...source, appeal: { ...source.appeal, outcome: 'reversed' } }));
  assert.throws(() => enforcementNoticeFromAPI({ ...source, action: 'suspension' }));
});
