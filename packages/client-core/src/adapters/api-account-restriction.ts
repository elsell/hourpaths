import { createSessionApiClient, observeAccountRestriction } from '@hourpaths/api-client';
import { validAccountRestrictionReview, type AccountRestrictionRepository, type AccountRestrictionReview } from '../account-restriction';
export function apiAccountRestriction(baseURL: string, token: () => string | null, expectedUserId?: string, rejected?: (token: string | null) => void): AccountRestrictionRepository {
  let owner = expectedUserId;
  return {
    observeRestricted: notify => observeAccountRestriction(baseURL, token, notify),
    async review(signal) {
      const credential = token();
      if (!credential) throw new Error('account_restriction_session_unavailable');
      const api = createSessionApiClient(baseURL, () => credential, signal, rejected);
      if (!owner) {
        const profile = await api.profile();
        if (!profile.response.ok || !profile.data?.data.id || token() !== credential) throw new Error('account_restriction_owner_unavailable');
        owner = profile.data.data.id;
      }
      const response = await api.reviewAccountRestriction();
      if (!response.response.ok || !response.data || token() !== credential || signal?.aborted) throw new Error('account_restriction_unavailable');
      const data = response.data.data.restriction;
      const review: AccountRestrictionReview = { userId: owner, restriction: data === null ? null : {
        noticeId: data.noticeId, action: data.action, reason: data.policyReason, issuedAt: data.issuedAt,
        ...(data.until ? { until: data.until } : {}),
      } };
      if (!validAccountRestrictionReview(review)) throw new Error('account_restriction_invalid');
      return review;
    },
  };
}
