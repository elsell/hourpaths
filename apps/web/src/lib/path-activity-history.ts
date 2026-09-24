import { manualActivityParticipantNow, type ManualActivityParticipantNow } from '@hourpaths/client-core';

export type Activity = {
  id: string;
  pathId: string;
  participantId: string;
  startedAt: string;
  endedAt: string;
  durationSeconds: number;
  occurrenceTimeZone: string;
  createdAt: string;
  updatedAt: string;
  note?: string;
};

export type ActivityHistoryItem = { activity: Activity; version: number };
export type RevisionHistoryItem = { version: number };

export type ActivityDay = {
  localDate: string;
  items: ActivityHistoryItem[];
};

function retainedLocalDate(activity: Activity): string {
  const parts = new Intl.DateTimeFormat('en', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: activity.occurrenceTimeZone,
  }).formatToParts(new Date(activity.startedAt));
  const value = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? '';
  return `${value('year')}-${value('month')}-${value('day')}`;
}

export function groupActivitiesNewestFirst(items: ActivityHistoryItem[]): ActivityDay[] {
  const grouped: Record<string, ActivityHistoryItem[]> = {};
  for (const item of [...items].sort((left, right) => Date.parse(right.activity.startedAt) - Date.parse(left.activity.startedAt))) {
    const localDate = retainedLocalDate(item.activity);
    grouped[localDate] = [...(grouped[localDate] ?? []), item];
  }
  return Object.entries(grouped)
    .sort(([left], [right]) => right.localeCompare(left))
    .map(([localDate, groupedItems]) => ({ localDate, items: groupedItems }));
}

export function mergeActivityHistory(
  current: ActivityHistoryItem[],
  incoming: ActivityHistoryItem[],
  replace = false,
): ActivityHistoryItem[] {
  const merged = replace ? [] : [...current];
  const seen = new Set(merged.map((item) => item.activity.id));
  for (const item of incoming) {
    if (seen.has(item.activity.id)) continue;
    seen.add(item.activity.id);
    merged.push(item);
  }
  return merged;
}

export function removeActivity(items: ActivityHistoryItem[], activityID: string): ActivityHistoryItem[] {
  const index = items.findIndex((item) => item.activity.id === activityID);
  if (index === -1) return items;
  return [...items.slice(0, index), ...items.slice(index + 1)];
}

export function mergeRevisionHistory<T extends RevisionHistoryItem>(current: T[], incoming: T[], replace = false): T[] {
  const merged = replace ? [] : [...current];
  const seen = new Set(merged.map((revision) => revision.version));
  for (const revision of incoming) {
    if (seen.has(revision.version)) continue;
    seen.add(revision.version);
    merged.push(revision);
  }
  return merged;
}

export function ownsActivity(activity: Activity, profileID: string): boolean {
  return activity.participantId === profileID;
}

export function activityEditForm(activity: Activity): { localDate: string; localTime: string; durationSeconds: string; occurrenceTouched: true } {
  const parts = new Intl.DateTimeFormat('en', {
    timeZone: activity.occurrenceTimeZone,
    calendar: 'iso8601',
    numberingSystem: 'latn',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(new Date(activity.startedAt));
  const value = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? '';
  return {
    localDate: `${value('year')}-${value('month')}-${value('day')}`,
    localTime: `${value('hour')}:${value('minute')}:${value('second')}`,
    durationSeconds: String(activity.durationSeconds),
    occurrenceTouched: true,
  };
}

export function activityValidationNow(
  currentInstant: string,
  profileTimeZone: string,
  elapsedMilliseconds: number,
  retainedOccurrenceTimeZone?: string,
): ManualActivityParticipantNow {
  return manualActivityParticipantNow(
    currentInstant,
    retainedOccurrenceTimeZone ?? profileTimeZone,
    elapsedMilliseconds,
  );
}
