export const durationUnits = ['seconds', 'minutes', 'hours'] as const;

export type DurationUnit = (typeof durationUnits)[number];

const secondsPerUnit: Record<DurationUnit, number> = {
  seconds: 1,
  minutes: 60,
  hours: 3_600,
};

export function durationSecondsForInput(value: string, unit: DurationUnit): string {
  if (value === '') return '';
  if (!/^\d+$/.test(value)) return value;
  const result = Number(value) * secondsPerUnit[unit];
  return Number.isSafeInteger(result) ? String(result) : String(Number.MAX_SAFE_INTEGER + 1);
}

export function durationInputForSeconds(
  seconds: string,
  fallbackUnit: DurationUnit,
): { unit: DurationUnit; value: string } {
  if (!/^\d+$/.test(seconds)) return { unit: fallbackUnit, value: seconds };
  const total = Number(seconds);
  if (!Number.isSafeInteger(total)) return { unit: fallbackUnit, value: seconds };
  for (const unit of ['hours', 'minutes', 'seconds'] as const) {
    const factor = secondsPerUnit[unit];
    if (total % factor === 0) {
      return { unit, value: String(total / factor) };
    }
  }
  return { unit: 'seconds', value: seconds };
}
