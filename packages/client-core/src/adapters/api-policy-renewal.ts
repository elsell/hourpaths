import { createSessionApiClient, observePolicyRequirement } from '@hourpaths/api-client';
import { PolicyRenewalFailure, validPolicyReview, type PolicyRenewalRepository, type PolicyReview } from '../policy-renewal';
export function apiPolicyRenewal(baseURL: string, token: string | null | (() => string | null), expectedUserId?: string, rejected?: (token: string | null) => void): PolicyRenewalRepository {
  let userId = expectedUserId;
  const credential = () => typeof token === 'function' ? token() : token;
  async function identify(captured: string | null, signal?: AbortSignal) {
    if (!captured) throw new PolicyRenewalFailure('rejected');
    if (userId) return userId;
    const result = await createSessionApiClient(baseURL, () => captured, signal, rejected).profile();
    if (!result.response.ok || !result.data) throw failure(result.response.status, result.error);
    if (credential() !== captured || !result.data.data.id) throw new PolicyRenewalFailure();
    userId = result.data.data.id;
    return userId;
  }
  function failure(status: number, problem: unknown) {
    const code = typeof problem === 'object' && problem !== null && 'code' in problem ? problem.code : undefined;
    return new PolicyRenewalFailure(status === 409 && code === 'policy_set_changed' ? 'review_changed' : status === 401 ? 'rejected' : status === 400 || status === 422 ? 'invalid' : 'unavailable');
  }
  return {
    observeRequired: notify => observePolicyRequirement(baseURL, credential, notify),
    async review(signal) {
      const captured = credential();
      const owner = await identify(captured, signal);
      const result = await createSessionApiClient(baseURL, () => captured, signal, rejected).reviewCurrentPolicies();
      if (!result.response.ok || !result.data) throw failure(result.response.status, result.error);
      const data = result.data.data;
      if (credential() !== captured) throw new PolicyRenewalFailure();
      const review: PolicyReview = { userId: owner, required: data.required, token: data.reviewToken, policies: {
        terms: { version: data.policies.termsOfService.version, url: data.policies.termsOfService.url },
        privacy: { version: data.policies.privacyPolicy.version, url: data.policies.privacyPolicy.url },
        guidelines: { version: data.policies.communityGuidelines.version, url: data.policies.communityGuidelines.url },
        supportURL: data.policies.supportUrl,
      } };
      if (!validPolicyReview(review)) throw new PolicyRenewalFailure();
      return review;
    },
    async accept(input, key, signal) {
      if (!userId || input.review.userId !== userId || !validPolicyReview(input.review) || !input.termsAccepted || !input.privacyAcknowledged || !input.guidelinesAccepted) throw new PolicyRenewalFailure('invalid');
      const result = await createSessionApiClient(baseURL, credential, signal, rejected).acceptCurrentPolicies({ reviewToken: input.review.token, termsAccepted: input.termsAccepted, privacyAcknowledged: input.privacyAcknowledged, communityGuidelinesAccepted: input.guidelinesAccepted }, key);
      if (!result.response.ok || !result.data) throw failure(result.response.status, result.error);
      const data = result.data.data;
      return { userId, termsVersion: data.termsVersion, privacyVersion: data.privacyPolicyVersion, guidelinesVersion: data.communityGuidelinesVersion, acceptedAt: data.acceptedAt };
    },
  };
}
