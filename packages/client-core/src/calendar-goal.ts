import { localDateTimeValue, participantInstantValue, participantLocalDateTime } from './calendar-time';

export interface CalendarGoal {
  targetSeconds: number;
  recurrence: 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly';
  alignment: { minute?: number; hour?: number; isoWeekday?: number; month?: number; day?: number };
}
function wall(year: number, month: number, day: number, hour = 0, minute = 0): Date {
  const value = new Date(0); value.setUTCFullYear(year, month, day); value.setUTCHours(hour, minute, 0, 0); return value;
}
function days(year: number, month: number) { return wall(year, month + 1, 0).getUTCDate(); }

/** Same half-open participant-calendar boundaries as the API. Resolve each
 * nearby wall boundary, then deduplicate gaps before selecting the interval. */
export function currentGoalPeriod(goal: CalendarGoal, zone: string, now: number): { startsAt: number; endsAt: number; targetSeconds: number } {
  const invalid = () => new Error('tracking_goal_invalid');
  const a = { minute: goal.alignment.minute ?? 0, hour: goal.alignment.hour ?? 0, isoWeekday: goal.alignment.isoWeekday ?? 0, month: goal.alignment.month ?? 0, day: goal.alignment.day ?? 0 };
  const range = (n: number, min: number, max: number) => Number.isInteger(n) && n >= min && n <= max;
  const valid = Object.values(a).every(Number.isInteger) && Number.isSafeInteger(goal.targetSeconds) && goal.targetSeconds > 0 && (
    goal.recurrence === 'hourly' ? range(a.minute, 0, 59) && !a.hour && !a.isoWeekday && !a.month && !a.day :
    goal.recurrence === 'daily' ? range(a.hour, 0, 23) && !a.minute && !a.isoWeekday && !a.month && !a.day :
    goal.recurrence === 'weekly' ? range(a.isoWeekday, 1, 7) && !a.minute && !a.hour && !a.month && !a.day :
    goal.recurrence === 'monthly' ? range(a.day, 1, 31) && !a.minute && !a.hour && !a.isoWeekday && !a.month :
    goal.recurrence === 'yearly' ? range(a.month, 1, 12) && range(a.day, 1, days(2000, a.month - 1)) && !a.minute && !a.hour && !a.isoWeekday : false);
  if (!valid) throw invalid();
  const local = participantLocalDateTime(now, zone);
  if (!local) throw invalid();
  const nominal = localDateTimeValue(local);
  if (nominal === undefined) throw invalid();
  const date = new Date(nominal), year = date.getUTCFullYear(), month = date.getUTCMonth(), day = date.getUTCDate();
  const instants = new Set<number>();
  for (let offset = -3; offset <= 3; offset++) {
    let boundary: Date;
    switch (goal.recurrence) {
      case 'hourly': boundary = wall(year, month, day, date.getUTCHours() + offset, a.minute); break;
      case 'daily': boundary = wall(year, month, day + offset, a.hour); break;
      case 'weekly': boundary = wall(year, month, day + a.isoWeekday - (date.getUTCDay() || 7) + offset * 7); break;
      case 'monthly': {
        const shifted = wall(year, month + offset, 1);
        boundary = wall(shifted.getUTCFullYear(), shifted.getUTCMonth(), Math.min(a.day, days(shifted.getUTCFullYear(), shifted.getUTCMonth()))); break;
      }
      case 'yearly': boundary = wall(year + offset, a.month - 1, Math.min(a.day, days(year + offset, a.month - 1))); break;
    }
    const iso = boundary.toISOString();
    const instant = participantInstantValue({ localDate: iso.slice(0, 10), localTime: iso.slice(11, 19) }, zone);
    if (instant === undefined) throw invalid();
    instants.add(instant);
  }
  const ordered = [...instants].sort((a, b) => a - b);
  const next = ordered.findIndex(instant => instant > now);
  if (next <= 0) throw invalid();
  return { startsAt: ordered[next - 1], endsAt: ordered[next], targetSeconds: goal.targetSeconds };
}
