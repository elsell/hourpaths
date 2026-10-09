export type EnforcementAction = 'warning' | 'content_removal' | 'suspension' | 'ban';
export interface EnforcementAppeal {
  id: string; explanation: string; submittedAt: string;
  outcome?: 'upheld' | 'reversed'; decisionReason?: string; decidedAt?: string;
}
export interface EnforcementNotice {
  affectedComment?: { id: string; createdAt: string };
  id: string; action: EnforcementAction; policyReason: string; issuedAt: string;
  until?: string; appealDeadline: string; appeal?: EnforcementAppeal;
}
export interface EnforcementRepository {
  list(cursor?: string): Promise<{ items: EnforcementNotice[]; nextCursor?: string }>;
  get(id: string): Promise<EnforcementNotice>;
  appeal(id: string, explanation: string, key: string): Promise<EnforcementAppeal>;
}
export class EnforcementFailure extends Error {
  constructor(readonly kind: 'unavailable' | 'rejected' | 'not_found' | 'conflict' | 'invalid' = 'unavailable') { super(`enforcement_${kind}`); }
}
export function appealAvailable(notice: EnforcementNotice, now: number): boolean {
  return !notice.appeal && Number.isFinite(now) && now >= Date.parse(notice.issuedAt) && now < Date.parse(notice.appealDeadline);
}
/** Account-owned submission; ambiguous retries retain their original identity. */
export function createAppealSubmissionOwner(newKey: () => string) {
  let epoch = 0, active: number | null = null;
  let retry: { id: string; explanation: string; key: string } | undefined;
  return {
    async submit(id: string, explanation: string, request: EnforcementRepository['appeal']): Promise<
      { kind: 'submitted'; appeal: EnforcementAppeal } | { kind: 'failed'; cause: unknown } | { kind: 'busy' | 'superseded' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch; active = generation;
      try {
        if (!id || id.trim() !== id) throw new EnforcementFailure('invalid');
        const normalized = explanation.trim();
        const attempt = retry?.id === id && retry.explanation === normalized ? retry : { id, explanation: normalized, key: newKey() };
        retry = attempt;
        const appeal = await request(id, normalized, attempt.key);
        if (generation !== epoch) return { kind: 'superseded' };
        if (appeal.id !== attempt.key || appeal.explanation !== normalized || !Number.isFinite(Date.parse(appeal.submittedAt))) throw new EnforcementFailure();
        retry = undefined;
        return { kind: 'submitted', appeal: { ...appeal } };
      } catch (cause) { return generation === epoch ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
