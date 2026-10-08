export type TimerSubscriptionScope = 'person' | 'path';
export interface TimerSubscriptionSubject { scope: TimerSubscriptionScope; id: string }
export interface TimerSubscriptionPreference { enabled: boolean; revision: number }
export interface TimerSubscriptionsRepository {
  get(subject: TimerSubscriptionSubject): Promise<TimerSubscriptionPreference>;
  update(subject: TimerSubscriptionSubject, preference: TimerSubscriptionPreference, idempotencyKey: string): Promise<TimerSubscriptionPreference>;
}
export class TimerSubscriptionFailure extends Error {
  constructor(readonly kind: 'conflict' | 'rejected' | 'unavailable' | 'not_found' = 'unavailable') { super(`timer_subscription_${kind}`); }
}

/** One mounted account/subject owns each operation and its ambiguous retry. */
export function createTimerSubscriptionOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(subject: TimerSubscriptionSubject, value: TimerSubscriptionPreference, request: (subject: TimerSubscriptionSubject, value: TimerSubscriptionPreference, key: string) => Promise<TimerSubscriptionPreference>): Promise<
      { kind: 'applied'; preference: TimerSubscriptionPreference } | { kind: 'failed'; cause: unknown } | { kind: 'superseded' | 'busy' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch;
      active = generation;
      const target = { ...subject }, frozen = { ...value };
      const signature = JSON.stringify([target.scope, target.id, frozen.revision, frozen.enabled]);
      const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() };
      retry = attempt;
      try {
        const preference = await request(target, frozen, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (preference.enabled !== frozen.enabled || preference.revision !== frozen.revision + 1) throw new TimerSubscriptionFailure();
        retry = undefined;
        return { kind: 'applied', preference };
      } catch (cause) {
        return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
