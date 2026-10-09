export interface PolicyReference { version: string; url: string }
export interface PolicyReview {
  userId: string;
  required: boolean;
  token: string;
  policies: { terms: PolicyReference; privacy: PolicyReference; guidelines: PolicyReference; supportURL: string };
}
export interface PolicyConfirmation { review: PolicyReview; termsAccepted: boolean; privacyAcknowledged: boolean; guidelinesAccepted: boolean }
export interface PolicyAcceptance { userId: string; termsVersion: string; privacyVersion: string; guidelinesVersion: string; acceptedAt: string }
export interface PolicyRenewalRepository {
  observeRequired?(notify: () => void): () => void;
  review(signal?: AbortSignal): Promise<PolicyReview>;
  accept(input: PolicyConfirmation, key: string, signal?: AbortSignal): Promise<PolicyAcceptance>;
}
export class PolicyRenewalFailure extends Error {
  constructor(readonly kind: 'invalid' | 'review_changed' | 'rejected' | 'unavailable' = 'unavailable') { super(`policy_renewal_${kind}`); }
}
export function validPolicyReview(review: PolicyReview): boolean {
  const safeURL = (value: string) => { try { const url = new URL(value); return ['https:', 'http:'].includes(url.protocol) && !!url.hostname && !url.username && !url.password && !url.search && !url.hash; } catch { return false; } };
  return !!review.userId && !!review.token && typeof review.required === 'boolean' && [review.policies.terms, review.policies.privacy, review.policies.guidelines].every(policy => !!policy.version.trim() && safeURL(policy.url)) && safeURL(review.policies.supportURL);
}
export function createPolicyRenewalOwner(userId: string, keyFactory: () => string) {
  let epoch = 0, disposed = false, active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(input: PolicyConfirmation, save: (value: PolicyConfirmation, key: string) => Promise<PolicyAcceptance>): Promise<
      { kind: 'applied'; acceptance: PolicyAcceptance } | { kind: 'failed'; cause: unknown } | { kind: 'busy' | 'superseded' }
    > {
      if (disposed) return { kind: 'superseded' };
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch; active = generation;
      try {
        const policies = input.review.policies;
        const frozen: PolicyConfirmation = { ...input, review: { ...input.review, policies: { terms: { ...policies.terms }, privacy: { ...policies.privacy }, guidelines: { ...policies.guidelines }, supportURL: policies.supportURL } } };
        if (!userId || frozen.review.userId !== userId || !validPolicyReview(frozen.review) || !frozen.termsAccepted || !frozen.privacyAcknowledged || !frozen.guidelinesAccepted) throw new PolicyRenewalFailure('invalid');
        const signature = JSON.stringify([userId, frozen.review.token, frozen.review.policies]);
        const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() }; retry = attempt;
        const acceptance = await save(frozen, attempt.key);
        if (disposed || epoch !== generation) return { kind: 'superseded' };
        if (acceptance.userId !== userId || acceptance.termsVersion !== frozen.review.policies.terms.version || acceptance.privacyVersion !== frozen.review.policies.privacy.version || acceptance.guidelinesVersion !== frozen.review.policies.guidelines.version || !Number.isFinite(Date.parse(acceptance.acceptedAt))) throw new PolicyRenewalFailure();
        retry = undefined; return { kind: 'applied', acceptance };
      } catch (cause) { return !disposed && epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (active === generation) active = null; }
    },
    cancel() { disposed = true; epoch++; active = null; retry = undefined; },
  };
}
