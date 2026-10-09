export interface GoalReminderSubject { id: string }
export interface GoalReminderPreference { enabled: boolean; revision: number }
export interface GoalRemindersRepository {
  get(subject: GoalReminderSubject): Promise<GoalReminderPreference>;
  update(subject: GoalReminderSubject, preference: GoalReminderPreference, idempotencyKey: string): Promise<GoalReminderPreference>;
}
export class GoalReminderFailure extends Error {
  constructor(readonly kind: 'conflict' | 'rejected' | 'unavailable' | 'not_found' = 'unavailable') { super(`goal_reminder_${kind}`); }
}

/** One mounted account/subject owns each operation and its ambiguous retry. */
export function createGoalReminderOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(subject: GoalReminderSubject, value: GoalReminderPreference, request: (subject: GoalReminderSubject, value: GoalReminderPreference, key: string) => Promise<GoalReminderPreference>): Promise<
      { kind: 'applied'; preference: GoalReminderPreference } | { kind: 'failed'; cause: unknown } | { kind: 'superseded' | 'busy' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch;
      active = generation;
      const target = { ...subject }, frozen = { ...value };
      const signature = JSON.stringify([target.id, frozen.revision, frozen.enabled]);
      const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() };
      retry = attempt;
      try {
        const preference = await request(target, frozen, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (preference.enabled !== frozen.enabled || preference.revision !== frozen.revision + 1) throw new GoalReminderFailure();
        retry = undefined;
        return { kind: 'applied', preference };
      } catch (cause) {
        return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
