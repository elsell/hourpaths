import assert from 'node:assert/strict';
import test from 'node:test';
import { PolicyReviewController } from './policy-review-controller';
import { PolicyRenewalFailure, type PolicyReview } from './policy-renewal';
const review: PolicyReview = { userId: 'alice', required: true, token: 'first', policies: { terms: { version: 't1', url: 'https://app.test/t' }, privacy: { version: 'p1', url: 'https://app.test/p' }, guidelines: { version: 'g1', url: 'https://app.test/g' }, supportURL: 'https://app.test/help' } };
test('policy review refreshes changed publications and preserves the requirement through a failed refresh', async () => {
 let published = review, unavailable = false;
 const controller = new PolicyReviewController({ review: async () => { if (unavailable) throw new Error('offline'); return published; }, accept: async () => { throw new PolicyRenewalFailure('review_changed'); } }, () => 'key');
 await controller.refresh(); assert.equal(controller.state.required, true);
 published = { ...review, token: 'second' };
 await controller.accept({ termsAccepted: true, privacyAcknowledged: true, guidelinesAccepted: true });
 assert.equal(controller.state.review?.token, 'second'); assert.equal(controller.state.error, 'review_changed');
 unavailable = true; await controller.refresh(); assert.equal(controller.state.required, true);
 unavailable = false; published = { ...published, required: false }; await controller.refresh(); assert.equal(controller.state.required, false);
 controller.dispose();
});
test('disposing a policy controller discards a delayed review without publishing another account state', async () => {
 let complete!: (value: PolicyReview) => void;
 const controller = new PolicyReviewController({ review: () => new Promise(resolve => { complete = resolve; }), accept: async () => { throw new Error('unused'); } }, () => 'key');
 const pending = controller.refresh(); controller.dispose(); complete(review); await pending;
 assert.equal(controller.state.review, null);
});
