export interface WeekStartPreference { userId: string; firstDayOfWeek: number }
export interface WeekStartChange { userId: string; reviewedFirstDayOfWeek: number; proposedFirstDayOfWeek: number }
export interface WeekStartPreferenceRepository {
  read(signal?: AbortSignal): Promise<WeekStartPreference>;
  save(change: WeekStartChange, key: string, signal?: AbortSignal): Promise<WeekStartPreference>;
}
export class WeekStartFailure extends Error {
  constructor(readonly kind: 'invalid' | 'conflict' | 'rejected' | 'unavailable' = 'unavailable') { super(`week_start_${kind}`); }
}
export function validWeekday(value: number) { return Number.isInteger(value) && value >= 1 && value <= 7; }
export function createWeekStartOperationOwner(userId: string, keyFactory: () => string) {
  let epoch = 0, active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(change: WeekStartChange, save: (value: WeekStartChange, key: string) => Promise<WeekStartPreference>): Promise<
      { kind: 'applied'; preference: WeekStartPreference } | { kind: 'failed'; cause: unknown } | { kind: 'busy' | 'superseded' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch; active = generation;
      try {
        const frozen = { ...change };
        if (!userId || frozen.userId !== userId || !validWeekday(frozen.reviewedFirstDayOfWeek) || !validWeekday(frozen.proposedFirstDayOfWeek)) throw new WeekStartFailure('invalid');
        const signature = JSON.stringify([userId, frozen.reviewedFirstDayOfWeek, frozen.proposedFirstDayOfWeek]);
        const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() }; retry = attempt;
        const preference = await save(frozen, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (preference.userId !== userId || preference.firstDayOfWeek !== frozen.proposedFirstDayOfWeek) throw new WeekStartFailure();
        retry = undefined; return { kind: 'applied', preference };
      } catch (cause) { return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (active === generation) active = null; }
    },
    cancel() { epoch++; active = null; retry = undefined; },
  };
}
