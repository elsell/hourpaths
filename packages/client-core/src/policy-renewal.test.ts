import assert from 'node:assert/strict';
import test from 'node:test';
import { createPolicyRenewalOwner, type PolicyReview, type PolicyAcceptance } from './policy-renewal';
const review: PolicyReview = { userId: 'alice', required: true, token: 'signed-review', policies: {
 terms: { version: 'terms2', url: 'https://app.example/terms' }, privacy: { version: 'privacy1', url: 'https://app.example/privacy' }, guidelines: { version: 'guidelines1', url: 'https://app.example/guidelines' }, supportURL: 'https://app.example/support',
} };
const result: PolicyAcceptance = { userId: 'alice', termsVersion: 'terms2', privacyVersion: 'privacy1', guidelinesVersion: 'guidelines1', acceptedAt: '2026-10-09T00:00:00Z' };
test('policy acceptance retries retain reviewed evidence and cancelled account responses are discarded', async () => {
 let keys = 0; const owner = createPolicyRenewalOwner('alice', () => `key-${++keys}`);
 const input = { review, termsAccepted: true, privacyAcknowledged: true, guidelinesAccepted: true };
 const received: string[] = [];
 assert.equal((await owner.submit(input, async (_, key) => { received.push(key); throw new Error('lost response'); })).kind, 'failed');
 let complete!: (value: PolicyAcceptance) => void;
 const pending = owner.submit(input, (_, key) => { received.push(key); return new Promise(resolve => { complete = resolve; }); });
 assert.equal((await owner.submit(input, async () => result)).kind, 'busy');
 owner.cancel(); complete(result);
 assert.equal((await pending).kind, 'superseded');
 assert.deepEqual(received, ['key-1', 'key-1']);
 assert.equal((await owner.submit(input, async () => { throw new Error('disposed owner submitted'); })).kind, 'superseded');
});
test('policy confirmation cannot submit another account, incomplete acceptance, or accept mismatched versions', async () => {
 const owner = createPolicyRenewalOwner('alice', () => 'key');
 let calls = 0; const save = async () => { calls++; return result; };
 const input = { review, termsAccepted: true, privacyAcknowledged: true, guidelinesAccepted: true };
 assert.equal((await owner.submit({ ...input, privacyAcknowledged: false }, save)).kind, 'failed');
 assert.equal((await owner.submit({ ...input, review: { ...review, userId: 'bob' } }, save)).kind, 'failed');
 assert.equal(calls, 0);
 assert.equal((await owner.submit(input, async () => ({ ...result, termsVersion: 'unreviewed' }))).kind, 'failed');
 assert.equal((await owner.submit(input, save)).kind, 'applied');
});
