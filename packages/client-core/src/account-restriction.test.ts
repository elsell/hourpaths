import assert from 'node:assert/strict';
import test from 'node:test';
import { AccountRestrictionController, type AccountRestrictionReview } from './account-restriction';
const restricted: AccountRestrictionReview = { userId: 'alice', restriction: { noticeId: 'notice', action: 'suspension', reason: 'Reviewed policy violation', issuedAt: '2026-10-13T00:00:00Z', until: '2026-10-14T00:00:00Z' } };
test('restriction survives connectivity failure and clears only after authoritative review', async () => {
 let result = restricted, offline = false;
 const controller = new AccountRestrictionController({ review: async () => { if (offline) throw new Error('offline'); return result; } });
 await controller.refresh(); assert.equal(controller.state.required, true);
 offline = true; await controller.refresh(); assert.equal(controller.state.required, true); assert.equal(controller.state.error, 'unavailable');
 offline = false; result = { userId: 'alice', restriction: null }; await controller.refresh(); assert.equal(controller.state.required, false);
 controller.dispose();
});
test('transport denial invalidates an older unrestricted response and retains the restriction until new review', async () => {
 const pending: Array<(review: AccountRestrictionReview) => void> = [];
 let notify!: () => void;
 const controller = new AccountRestrictionController({ review: () => new Promise(resolve => pending.push(resolve)), observeRestricted: callback => { notify = callback; return () => {}; } });
 const old = controller.refresh(); notify(); assert.equal(controller.state.required, true);
 pending[0]({ userId: 'alice', restriction: null }); await old;
 assert.equal(controller.state.required, true);
 pending[1](restricted); await new Promise(resolve => setImmediate(resolve));
 assert.equal(controller.state.review?.restriction?.noticeId, 'notice'); controller.dispose();
});
test('a foreign or invalid review never clears a known restriction; disposal suppresses delayed updates', async () => {
 let result = restricted;
 const controller = new AccountRestrictionController({ review: async () => result });
 await controller.refresh(); result = { userId: 'bob', restriction: null }; await controller.refresh();
 assert.equal(controller.state.required, true); assert.equal(controller.state.review?.userId, 'alice');
 let complete!: (review: AccountRestrictionReview) => void;
 const delayed = new AccountRestrictionController({ review: () => new Promise(resolve => { complete = resolve; }) });
 let updates = 0; delayed.subscribe(() => updates++);
 const work = delayed.refresh(); delayed.dispose(); const before = updates; complete(restricted); await work;
 assert.equal(updates, before); assert.equal(delayed.state.review, null); controller.dispose();
});

test('same-account credential rotation retains an already known restriction while review is unavailable', async () => {
 const next = new AccountRestrictionController({ review: async () => { throw new Error('offline'); } }, restricted);
 assert.equal(next.state.required, true); await next.refresh(); assert.equal(next.state.required, true);
 next.dispose();
});
