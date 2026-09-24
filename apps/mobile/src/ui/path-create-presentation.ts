import type { PathGoalForm } from '../path-goals';

export {
  durationInputForSeconds,
  durationSecondsForInput,
  durationUnits as goalDurationUnits,
  type DurationUnit as GoalDurationUnit,
} from './duration-input';

export function pathAlignmentIsCustomized(form: PathGoalForm): boolean {
  switch (form.recurrence) {
    case 'hourly':
      return form.hourlyMinute !== '0';
    case 'daily':
      return form.dailyHour !== '0';
    case 'weekly':
      return form.weeklyISOWeekday !== '';
    case 'monthly':
      return form.monthlyDay !== '1';
    case 'yearly':
      return form.yearlyMonth !== '1' || form.yearlyDay !== '1';
  }
}

export function resetPathAlignment(form: PathGoalForm): Partial<PathGoalForm> {
  switch (form.recurrence) {
    case 'hourly':
      return { hourlyMinute: '0' };
    case 'daily':
      return { dailyHour: '0' };
    case 'weekly':
      return { weeklyISOWeekday: '' };
    case 'monthly':
      return { monthlyDay: '1' };
    case 'yearly':
      return { yearlyMonth: '1', yearlyDay: '1' };
  }
}
