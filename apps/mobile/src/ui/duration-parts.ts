export type DurationParts = { hours: string; minutes: string; seconds: string };

export function durationParts(value: string): DurationParts {
  if (value === '') return { hours: '', minutes: '', seconds: '' };
  if (!/^\d+$/.test(value) || !Number.isSafeInteger(Number(value))) {
    return { hours: value, minutes: '', seconds: '' };
  }
  const total = Number(value);
  return {
    hours: Math.floor(total / 3600) ? String(Math.floor(total / 3600)) : '',
    minutes: Math.floor(total / 60) % 60 ? String(Math.floor(total / 60) % 60) : total === 0 ? '0' : '',
    seconds: total % 60 ? String(total % 60) : '',
  };
}

export function secondsFromDurationParts(parts: DurationParts): string {
  const fields = [parts.hours, parts.minutes, parts.seconds];
  if (fields.every((value) => value === '')) return '';
  if (fields.some((value) => value !== '' && !/^\d+$/.test(value))) return 'invalid';
  const total = Number(parts.hours) * 3600 + Number(parts.minutes) * 60 + Number(parts.seconds);
  return Number.isSafeInteger(total) ? String(total) : 'invalid';
}
