export const reportReasons = ['spam_or_scam', 'harassment_or_bullying', 'hate_or_abusive_content', 'sexual_or_inappropriate_content', 'impersonation', 'privacy_or_personal_information', 'dangerous_or_self_harm_content', 'something_else'] as const;
export type ReportReason = typeof reportReasons[number];
export interface ReportTarget { kind: 'profile' | 'path' | 'feed_event' | 'comment' | 'nudge'; id: string }
export interface ReportDraft { target: ReportTarget; reason: ReportReason; explanation: string }
export interface ReportBlockIdentity { userId: string; username: string; displayName: string }
export interface ReportReceipt { id: string; blockTarget?: ReportBlockIdentity }
export interface ReportingRepository { submit(draft: ReportDraft, key: string, signal?: AbortSignal): Promise<ReportReceipt> }
export class ReportFailure extends Error {
  constructor(readonly kind: 'invalid' | 'not_found' | 'rejected' | 'unavailable' = 'unavailable') { super(`report_${kind}`); }
}
export function normalizeReportDraft(value: ReportDraft): ReportDraft {
  const explanation = value.explanation.trim();
  if (!reportReasons.includes(value.reason) || Array.from(explanation).length > 1000 || !value.target.id || value.target.id.trim() !== value.target.id || value.target.id.length > 200 || !['profile', 'path', 'feed_event', 'comment', 'nudge'].includes(value.target.kind)) throw new ReportFailure('invalid');
  return { target: { ...value.target }, reason: value.reason, explanation };
}

/** A mounted account owns the draft, uncertain retry and post-submit block offer. */
export function createReportSubmissionOwner(newKey: () => string) {
  let epoch = 0, active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(draft: ReportDraft, request: ReportingRepository['submit']): Promise<
      { kind: 'submitted'; receipt: ReportReceipt } | { kind: 'failed'; cause: unknown } | { kind: 'busy' | 'superseded' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch; active = generation;
      try {
        const frozen = normalizeReportDraft(draft);
        const signature = JSON.stringify([frozen.target.kind, frozen.target.id, frozen.reason, frozen.explanation]);
        const attempt = retry?.signature === signature ? retry : { signature, key: newKey() };
        retry = attempt;
        const receipt = await request(frozen, attempt.key);
        if (generation !== epoch) return { kind: 'superseded' };
        if (!receipt.id || receipt.id.trim() !== receipt.id) throw new ReportFailure();
        retry = undefined;
        return { kind: 'submitted', receipt: { id: receipt.id, ...(receipt.blockTarget ? { blockTarget: { ...receipt.blockTarget } } : {}) } };
      } catch (cause) { return generation === epoch ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
