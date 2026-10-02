export type ManualActivityLocalDateTime = { localDate: string; localTime: string };
// Bounded formatter reuse avoids rebuilding zone rules for every candidate wall
// time. Resolved instants still depend solely on the supplied occurrence/zone.
const formatters = new Map<string, Intl.DateTimeFormat>();

const localDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const localTimePattern = /^(\d{2}):(\d{2}):(\d{2})$/;

export function localDateTimeValue(value: ManualActivityLocalDateTime): number | undefined {
  const date = localDatePattern.exec(value.localDate);
  const time = localTimePattern.exec(value.localTime);
  if (!date || !time) return undefined;
  const year = Number(date[1]);
  const month = Number(date[2]);
  const day = Number(date[3]);
  const hour = Number(time[1]);
  const minute = Number(time[2]);
  const second = Number(time[3]);
  const nominal = new Date(0);
  nominal.setUTCFullYear(year, month - 1, day);
  nominal.setUTCHours(hour, minute, second, 0);
  if (
    nominal.getUTCFullYear() !== year || nominal.getUTCMonth() !== month - 1 ||
    nominal.getUTCDate() !== day || nominal.getUTCHours() !== hour ||
    nominal.getUTCMinutes() !== minute || nominal.getUTCSeconds() !== second
  ) return undefined;
  return nominal.getTime();
}

export function participantLocalDateTime(value: number, timeZone: string): ManualActivityLocalDateTime | undefined {
  if (!Number.isFinite(value) || !timeZone || timeZone === 'Local' || timeZone.trim() !== timeZone) return undefined;
  try {
    let formatter = formatters.get(timeZone);
    if (!formatter) {
      formatter = new Intl.DateTimeFormat('en-CA', {
      timeZone,
      calendar: 'iso8601',
      numberingSystem: 'latn',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23',
      });
      if (formatters.size >= 32) formatters.delete(formatters.keys().next().value!);
      formatters.set(timeZone, formatter);
    }
    const parts = formatter.formatToParts(new Date(value));
    const part = (type: Intl.DateTimeFormatPartTypes) => parts.find((candidate) => candidate.type === type)?.value;
    const local = {
      localDate: `${part('year')}-${part('month')}-${part('day')}`,
      localTime: `${part('hour')}:${part('minute')}:${part('second')}`,
    };
    return localDateTimeValue(local) === undefined ? undefined : local;
  } catch {
    return undefined;
  }
}

export function participantInstantValue(value: ManualActivityLocalDateTime, timeZone: string): number | undefined {
  const wallValue = localDateTimeValue(value);
  if (wallValue === undefined) return undefined;
  const offsets = new Set<number>();
  for (let hour = -72; hour <= 72; hour += 1) {
    const sample = wallValue + hour * 3_600_000;
    const localSample = participantLocalDateTime(sample, timeZone);
    const localSampleValue = localSample && localDateTimeValue(localSample);
    if (localSampleValue !== undefined) offsets.add(localSampleValue - sample);
  }
  const exact: number[] = [];
  const forward: Array<{ instant: number; wall: number }> = [];
  for (const offset of offsets) {
    const instant = wallValue - offset;
    const candidate = participantLocalDateTime(instant, timeZone);
    const candidateWall = candidate && localDateTimeValue(candidate);
    if (candidateWall === wallValue) exact.push(instant);
    else if (candidateWall !== undefined && candidateWall > wallValue) forward.push({ instant, wall: candidateWall });
  }
  if (exact.length > 0) return Math.min(...exact);
  forward.sort((left, right) => left.wall - wallValue - (right.wall - wallValue) || left.instant - right.instant);
  return forward[0]?.instant;
}

