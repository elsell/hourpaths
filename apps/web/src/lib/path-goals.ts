import type {
  PathCreateDraft,
  PathGoalAlignment,
  PathIntervalGoalDraft,
  PathRecurrence,
  SessionPath,
} from '@hourpaths/api-client';

export type DurationFields = {
  hours: string;
  minutes: string;
  seconds: string;
};

export type GoalFormState = {
  intervalEnabled: boolean;
  intervalDuration: DurationFields;
  recurrence: PathRecurrence;
  customAlignment: boolean;
  alignmentValue: string;
  yearlyMonth: string;
  yearlyDay: string;
  overallEnabled: boolean;
  overallDuration: DurationFields;
};

export type GoalDraftResult =
  | { ok: true; goals: Pick<PathCreateDraft, 'intervalGoal' | 'overallTarget'> }
  | { ok: false; reason: 'duration' | 'alignment' };

function durationFields(totalSeconds: number): DurationFields {
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  const seconds = totalSeconds % 60;
  return { hours: String(hours), minutes: String(minutes), seconds: String(seconds) };
}

export function goalFormState(
  goals: Pick<SessionPath, 'intervalGoal' | 'overallTarget'>,
): GoalFormState {
  const interval = goals.intervalGoal;
  const alignment = interval?.alignment;
  const alignmentValue = interval?.recurrence === 'hourly' ? alignment?.minute
    : interval?.recurrence === 'daily' ? alignment?.hour
      : interval?.recurrence === 'weekly' ? alignment?.isoWeekday
        : interval?.recurrence === 'monthly' ? alignment?.day
          : undefined;
  return {
    intervalEnabled: interval !== undefined,
    intervalDuration: durationFields(interval?.targetSeconds ?? 0),
    recurrence: interval?.recurrence ?? 'daily',
    customAlignment: true,
    alignmentValue: String(alignmentValue ?? (interval ? '' : 0)),
    yearlyMonth: String(interval?.recurrence === 'yearly' ? alignment?.month ?? '' : 1),
    yearlyDay: String(interval?.recurrence === 'yearly' ? alignment?.day ?? '' : 1),
    overallEnabled: goals.overallTarget !== undefined,
    overallDuration: durationFields(goals.overallTarget?.targetSeconds ?? 0),
  };
}

function wholeNumber(value: string, maximum = Number.MAX_SAFE_INTEGER): number | null {
  if (!/^\d+$/.test(value)) return null;
  const number = Number(value);
  return Number.isSafeInteger(number) && number >= 0 && number <= maximum ? number : null;
}

function durationSeconds(fields: DurationFields): number | null {
  const hours = wholeNumber(fields.hours);
  const minutes = wholeNumber(fields.minutes, 59);
  const seconds = wholeNumber(fields.seconds, 59);
  if (hours === null || minutes === null || seconds === null) return null;
  const total = hours * 3600 + minutes * 60 + seconds;
  return Number.isSafeInteger(total) && total > 0 ? total : null;
}

function yearlyAlignment(monthText: string, dayText: string): PathGoalAlignment | null {
  const month = wholeNumber(monthText, 12);
  const day = wholeNumber(dayText, 31);
  if (month === null || month < 1 || day === null || day < 1) return null;
  const daysInMonth = new Date(Date.UTC(2024, month, 0)).getUTCDate();
  return day <= daysInMonth ? { month, day } : null;
}

function alignmentFor(state: GoalFormState): PathGoalAlignment | null {
  if (state.recurrence === 'yearly') return yearlyAlignment(state.yearlyMonth, state.yearlyDay);
  const limits: Record<Exclude<PathRecurrence, 'yearly'>, [number, number, keyof PathGoalAlignment]> = {
    hourly: [0, 59, 'minute'],
    daily: [0, 23, 'hour'],
    weekly: [1, 7, 'isoWeekday'],
    monthly: [1, 31, 'day'],
  };
  const [minimum, maximum, field] = limits[state.recurrence];
  const value = wholeNumber(state.alignmentValue, maximum);
  return value !== null && value >= minimum ? { [field]: value } : null;
}

export function buildGoalDraft(state: GoalFormState): GoalDraftResult {
  let intervalGoal: PathIntervalGoalDraft | undefined;
  if (state.intervalEnabled) {
    const targetSeconds = durationSeconds(state.intervalDuration);
    if (targetSeconds === null) return { ok: false, reason: 'duration' };
    if (state.customAlignment) {
      const alignment = alignmentFor(state);
      if (alignment === null) return { ok: false, reason: 'alignment' };
      intervalGoal = { targetSeconds, recurrence: state.recurrence, alignment };
    } else {
      intervalGoal = { targetSeconds, recurrence: state.recurrence };
    }
  }

  let overallTarget: { targetSeconds: number } | undefined;
  if (state.overallEnabled) {
    const targetSeconds = durationSeconds(state.overallDuration);
    if (targetSeconds === null) return { ok: false, reason: 'duration' };
    overallTarget = { targetSeconds };
  }

  return {
    ok: true,
    goals: {
      ...(intervalGoal ? { intervalGoal } : {}),
      ...(overallTarget ? { overallTarget } : {}),
    },
  };
}
