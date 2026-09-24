import type { PathCreateDraft, PathGoalAlignment, PathGoalUpdateDraft as GeneratedPathGoalUpdateDraft, PathRecurrence, SessionPath } from '@hourpaths/api-client';
import type { PathVisibility } from '@hourpaths/client-core';

export const pathRecurrences = ['hourly', 'daily', 'weekly', 'monthly', 'yearly'] as const satisfies readonly PathRecurrence[];

export type PathGoalForm = {
  intervalEnabled: boolean;
  intervalSeconds: string;
  recurrence: PathRecurrence;
  hourlyMinute: string;
  dailyHour: string;
  weeklyISOWeekday: string;
  monthlyDay: string;
  yearlyMonth: string;
  yearlyDay: string;
  overallEnabled: boolean;
  overallSeconds: string;
};

export type PathGoalFormResult =
  | { kind: 'valid'; draft: PathCreateDraft }
  | { kind: 'invalid_duration' }
  | { kind: 'invalid_alignment' };

export type PathGoalUpdateDraft = Pick<GeneratedPathGoalUpdateDraft, 'intervalGoal' | 'overallTarget'>;

export type PathGoalUpdateFormResult =
  | { kind: 'valid'; draft: PathGoalUpdateDraft }
  | { kind: 'invalid_duration' }
  | { kind: 'invalid_alignment' };

export function initialPathGoalForm(): PathGoalForm {
  return {
    intervalEnabled: false,
    intervalSeconds: '',
    recurrence: 'hourly',
    hourlyMinute: '0',
    dailyHour: '0',
    weeklyISOWeekday: '',
    monthlyDay: '1',
    yearlyMonth: '1',
    yearlyDay: '1',
    overallEnabled: false,
    overallSeconds: '',
  };
}

export function pathGoalFormFromPath(
  path: Pick<SessionPath, 'intervalGoal' | 'overallTarget'>,
): PathGoalForm {
  const form = initialPathGoalForm();
  const intervalGoal = path.intervalGoal;
  if (intervalGoal) {
    form.intervalEnabled = true;
    form.intervalSeconds = String(intervalGoal.targetSeconds);
    form.recurrence = intervalGoal.recurrence;
    switch (intervalGoal.recurrence) {
      case 'hourly':
        form.hourlyMinute = String(intervalGoal.alignment.minute ?? 0);
        break;
      case 'daily':
        form.dailyHour = String(intervalGoal.alignment.hour ?? 0);
        break;
      case 'weekly':
        form.weeklyISOWeekday = intervalGoal.alignment.isoWeekday === undefined
          ? ''
          : String(intervalGoal.alignment.isoWeekday);
        break;
      case 'monthly':
        form.monthlyDay = String(intervalGoal.alignment.day ?? 1);
        break;
      case 'yearly':
        form.yearlyMonth = String(intervalGoal.alignment.month ?? 1);
        form.yearlyDay = String(intervalGoal.alignment.day ?? 1);
        break;
    }
  }
  if (path.overallTarget) {
    form.overallEnabled = true;
    form.overallSeconds = String(path.overallTarget.targetSeconds);
  }
  return form;
}

function positiveWholeSeconds(value: string): number | null {
  if (!/^[1-9]\d*$/.test(value)) return null;
  const seconds = Number(value);
  return Number.isSafeInteger(seconds) ? seconds : null;
}

function boundedWholeNumber(value: string, minimum: number, maximum: number): number | null {
  if (!/^\d+$/.test(value)) return null;
  const number = Number(value);
  return Number.isSafeInteger(number) && number >= minimum && number <= maximum ? number : null;
}

function alignmentFor(form: PathGoalForm): PathGoalAlignment | null | undefined {
  switch (form.recurrence) {
    case 'hourly': {
      const minute = boundedWholeNumber(form.hourlyMinute, 0, 59);
      return minute === null ? null : { minute };
    }
    case 'daily': {
      const hour = boundedWholeNumber(form.dailyHour, 0, 23);
      return hour === null ? null : { hour };
    }
    case 'weekly': {
      if (form.weeklyISOWeekday === '') return undefined;
      const isoWeekday = boundedWholeNumber(form.weeklyISOWeekday, 1, 7);
      return isoWeekday === null ? null : { isoWeekday };
    }
    case 'monthly': {
      const day = boundedWholeNumber(form.monthlyDay, 1, 31);
      return day === null ? null : { day };
    }
    case 'yearly': {
      const month = boundedWholeNumber(form.yearlyMonth, 1, 12);
      const day = boundedWholeNumber(form.yearlyDay, 1, 31);
      if (month === null || day === null) return null;
      const date = new Date(Date.UTC(2000, month - 1, day));
      return date.getUTCMonth() === month - 1 && date.getUTCDate() === day ? { month, day } : null;
    }
  }
}

export function buildPathCreateDraft(
  name: string,
  form: PathGoalForm,
  visibility?: PathVisibility,
): PathGoalFormResult {
  const draft: PathCreateDraft = { name, ...(visibility ? { visibility } : {}) };
  if (form.intervalEnabled) {
    const targetSeconds = positiveWholeSeconds(form.intervalSeconds);
    if (targetSeconds === null) return { kind: 'invalid_duration' };
    const alignment = alignmentFor(form);
    if (alignment === null) return { kind: 'invalid_alignment' };
    draft.intervalGoal = {
      targetSeconds,
      recurrence: form.recurrence,
      ...(alignment === undefined ? {} : { alignment }),
    };
  }
  if (form.overallEnabled) {
    const targetSeconds = positiveWholeSeconds(form.overallSeconds);
    if (targetSeconds === null) return { kind: 'invalid_duration' };
    draft.overallTarget = { targetSeconds };
  }
  return { kind: 'valid', draft };
}

export function buildPathGoalUpdateDraft(form: PathGoalForm): PathGoalUpdateFormResult {
  const result = buildPathCreateDraft('goal-update', form);
  if (result.kind !== 'valid') return result;
  if (result.draft.intervalGoal && !result.draft.intervalGoal.alignment) {
    return { kind: 'invalid_alignment' };
  }
  return {
    kind: 'valid',
    draft: {
      ...(result.draft.intervalGoal ? {
        intervalGoal: {
          ...result.draft.intervalGoal,
          alignment: result.draft.intervalGoal.alignment!,
        },
      } : {}),
      ...(result.draft.overallTarget ? { overallTarget: result.draft.overallTarget } : {}),
    },
  };
}
