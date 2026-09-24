import type { components } from './schema';

export type PathRecurrence = components['schemas']['IntervalGoalCreate']['recurrence'];
export type PathGoalAlignment = components['schemas']['IntervalAlignment'];
export type PathIntervalGoalDraft = components['schemas']['IntervalGoalCreate'];
export type PathOverallTargetDraft = components['schemas']['OverallTarget'];
export type PathCreateBody = Omit<components['schemas']['PathCreate'], '$schema'>;
export type PathCreateDraft = PathCreateBody;
export type CreatedPath = components['schemas']['PathProjection'];

export type PathSubmissionResult =
  | { kind: 'created'; path: CreatedPath }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'invalid_goal' }
  | { kind: 'invalid_name' }
  | { kind: 'superseded' };

export type PathCreateRequest = (body: PathCreateBody, idempotencyKey: string) => Promise<CreatedPath>;

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

function exactKeys(value: object, expected: string[]): boolean {
  const keys = Object.keys(value).sort();
  return keys.length === expected.length && keys.every((key, index) => key === expected[index]);
}

function validPositiveSeconds(value: number): boolean {
  return Number.isSafeInteger(value) && value > 0;
}

function validIntegerBetween(value: number | undefined, minimum: number, maximum: number): boolean {
  return value !== undefined && Number.isSafeInteger(value) && value >= minimum && value <= maximum;
}

function validYearlyDate(month: number, day: number): boolean {
  const candidate = new Date(Date.UTC(2000, month - 1, day));
  return candidate.getUTCMonth() === month - 1 && candidate.getUTCDate() === day;
}

function validAlignment(recurrence: PathRecurrence, alignment: PathGoalAlignment): boolean {
  switch (recurrence) {
    case 'hourly':
      return exactKeys(alignment, ['minute']) && validIntegerBetween(alignment.minute, 0, 59);
    case 'daily':
      return exactKeys(alignment, ['hour']) && validIntegerBetween(alignment.hour, 0, 23);
    case 'weekly':
      return exactKeys(alignment, ['isoWeekday']) && validIntegerBetween(alignment.isoWeekday, 1, 7);
    case 'monthly':
      return exactKeys(alignment, ['day']) && validIntegerBetween(alignment.day, 1, 31);
    case 'yearly':
      return exactKeys(alignment, ['day', 'month'])
        && validIntegerBetween(alignment.month, 1, 12)
        && validIntegerBetween(alignment.day, 1, 31)
        && validYearlyDate(alignment.month!, alignment.day!);
    default:
      return false;
  }
}

function validGoals(draft: PathCreateDraft): boolean {
  if (draft.intervalGoal !== undefined) {
    const goal = draft.intervalGoal;
    const expectedKeys = goal.alignment === undefined
      ? ['recurrence', 'targetSeconds']
      : ['alignment', 'recurrence', 'targetSeconds'];
    if (!exactKeys(goal, expectedKeys)
      || !validPositiveSeconds(goal.targetSeconds)
      || !['hourly', 'daily', 'weekly', 'monthly', 'yearly'].includes(goal.recurrence)
      || (goal.alignment !== undefined && !validAlignment(goal.recurrence, goal.alignment))) {
      return false;
    }
  }
  return draft.overallTarget === undefined
    || (exactKeys(draft.overallTarget, ['targetSeconds'])
      && validPositiveSeconds(draft.overallTarget.targetSeconds));
}

function copyAlignment(alignment: PathGoalAlignment): PathGoalAlignment {
  return {
    ...(alignment.minute !== undefined ? { minute: alignment.minute } : {}),
    ...(alignment.hour !== undefined ? { hour: alignment.hour } : {}),
    ...(alignment.isoWeekday !== undefined ? { isoWeekday: alignment.isoWeekday } : {}),
    ...(alignment.month !== undefined ? { month: alignment.month } : {}),
    ...(alignment.day !== undefined ? { day: alignment.day } : {}),
  };
}

function canonicalBody(draft: PathCreateDraft): PathCreateBody {
  const intervalGoal = draft.intervalGoal === undefined ? undefined : {
    targetSeconds: draft.intervalGoal.targetSeconds,
    recurrence: draft.intervalGoal.recurrence,
    ...(draft.intervalGoal.alignment !== undefined
      ? { alignment: copyAlignment(draft.intervalGoal.alignment) }
      : draft.intervalGoal.recurrence === 'hourly'
        ? { alignment: { minute: 0 } }
        : draft.intervalGoal.recurrence === 'daily'
          ? { alignment: { hour: 0 } }
          : {}),
  };
  return {
    name: draft.name.trim(),
    ...(draft.visibility !== undefined ? { visibility: draft.visibility } : {}),
    ...(intervalGoal !== undefined ? { intervalGoal } : {}),
    ...(draft.overallTarget !== undefined ? {
      overallTarget: { targetSeconds: draft.overallTarget.targetSeconds },
    } : {}),
  };
}

function copyBody(body: PathCreateBody): PathCreateBody {
  return canonicalBody(body);
}

export function createPathSubmissionOwner(keyFactory: () => string) {
  let epoch = 0;
  let retry: { body: PathCreateBody; signature: string; idempotencyKey: string } | null = null;

  return {
    async submit(draft: PathCreateDraft, request: PathCreateRequest): Promise<PathSubmissionResult> {
      const body = canonicalBody(draft);
      if (!body.name) return { kind: 'invalid_name' };
      if (!validGoals(draft)) return { kind: 'invalid_goal' };
      const ownedEpoch = ++epoch;
      const signature = JSON.stringify(body);
      const matchingRetry = retry?.signature === signature ? retry : null;
      const idempotencyKey = matchingRetry?.idempotencyKey ?? keyFactory();
      if (!validIdempotencyKey(idempotencyKey)) throw new Error('invalid Path idempotency key');
      const retainedBody = matchingRetry?.body ?? copyBody(body);
      retry = { body: retainedBody, signature, idempotencyKey };
      try {
        const path = await request(copyBody(retainedBody), idempotencyKey);
        if (epoch !== ownedEpoch) return { kind: 'superseded' };
        retry = null;
        return { kind: 'created', path };
      } catch (cause) {
        return epoch === ownedEpoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      }
    },
    cancel() {
      epoch += 1;
      retry = null;
    },
  };
}
