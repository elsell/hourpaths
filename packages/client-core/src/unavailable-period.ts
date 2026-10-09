export interface UnavailablePeriodPreference {
  userId: string;
  enabled: boolean;
  startMinute: number;
  endMinute: number;
  revision: number;
  timeZone: string;
}
export interface UnavailablePeriodChange {
  userId: string;
  enabled: boolean;
  startMinute: number;
  endMinute: number;
  expectedRevision: number;
  reviewedTimeZone: string;
}
export interface UnavailablePeriodRepository {
  read(signal?: AbortSignal): Promise<UnavailablePeriodPreference>;
  save(change: UnavailablePeriodChange, key: string, signal?: AbortSignal): Promise<UnavailablePeriodPreference>;
}
export class UnavailablePeriodFailure extends Error {
  constructor(readonly kind: 'invalid' | 'conflict' | 'rejected' | 'unavailable' = 'unavailable') { super(`unavailable_period_${kind}`); }
}
export function validUnavailableMinutes(start: number, end: number) { return [start, end].every(value => Number.isInteger(value) && value >= 0 && value < 1440); }
export function createUnavailablePeriodOwner(userId: string, keyFactory: () => string) {
  let epoch = 0, active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(change: UnavailablePeriodChange, save: (value: UnavailablePeriodChange, key: string) => Promise<UnavailablePeriodPreference>): Promise<
      { kind: 'applied'; preference: UnavailablePeriodPreference } | { kind: 'failed'; cause: unknown } | { kind: 'busy' | 'superseded' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch; active = generation;
      try {
        const frozen = { ...change };
        if (!userId || frozen.userId !== userId || typeof frozen.enabled !== 'boolean' || !validUnavailableMinutes(frozen.startMinute, frozen.endMinute) || !Number.isSafeInteger(frozen.expectedRevision) || frozen.expectedRevision < 0 || !frozen.reviewedTimeZone) throw new UnavailablePeriodFailure('invalid');
        const signature = JSON.stringify([userId, frozen.enabled, frozen.startMinute, frozen.endMinute, frozen.expectedRevision, frozen.reviewedTimeZone]);
        const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() }; retry = attempt;
        const preference = await save(frozen, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (preference.userId !== userId || preference.enabled !== frozen.enabled || preference.startMinute !== frozen.startMinute || preference.endMinute !== frozen.endMinute || preference.revision !== frozen.expectedRevision + 1 || preference.timeZone !== frozen.reviewedTimeZone) throw new UnavailablePeriodFailure();
        retry = undefined; return { kind: 'applied', preference };
      } catch (cause) { return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (active === generation) active = null; }
    },
    cancel() { epoch++; active = null; retry = undefined; },
  };
}
