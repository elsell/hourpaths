import { manualActivityParticipantNow, type ManualActivityFormState } from '@hourpaths/client-core';

export type Activity = {
  id: string;
  pathId: string;
  participantId: string;
  startedAt: string;
  endedAt: string;
  occurrenceTimeZone: string;
  durationSeconds: number;
  note?: string;
  createdAt: string;
  updatedAt: string;
};
export type ActivityDetail = { activity: Activity; version: number };
export type ActivityRevision = Activity & { version: number; replacedAt: string };
export type ManualActivityDefaults = { currentInstant: string; localDate: string; localStartTime: string; timeZone: string };
export const retainedCalendarDayTimeZone = ['U', 'T', 'C'].join('');

export function retainedCalendarDayDate(localDate: string): Date {
  const [year, month, day] = localDate.split('-').map(Number);
  return new Date(Date.UTC(year, month - 1, day, 12));
}

export function pagedCollectionPresentation({
  busy,
  error,
  hasMore,
  itemCount,
}: {
  busy: boolean;
  error: boolean;
  hasMore: boolean;
  itemCount: number;
}) {
  return {
    showEmpty: itemCount === 0 && !busy && !error,
    showError: error,
    showLoadMore: hasMore && !error,
    showLoading: busy,
    showRetry: error,
  };
}

export function newestActivitiesFirst(items: readonly ActivityDetail[]): ActivityDetail[] {
  return [...items].sort((left, right) => {
    const byStart = Date.parse(right.activity.startedAt) - Date.parse(left.activity.startedAt);
    return byStart || right.activity.id.localeCompare(left.activity.id);
  });
}

export type ActivityDayGroup = { localDate: string; items: ActivityDetail[] };

export function appendUniqueActivities(
  existing: readonly ActivityDetail[],
  incoming: readonly ActivityDetail[],
): ActivityDetail[] {
  const merged = [...existing];
  for (const detail of incoming) {
    const index = merged.findIndex((current) => current.activity.id === detail.activity.id);
    if (index < 0) merged.push(detail);
    else merged[index] = detail;
  }
  return newestActivitiesFirst(merged);
}

export function appendUniqueRevisions(
  existing: readonly ActivityRevision[],
  incoming: readonly ActivityRevision[],
): ActivityRevision[] {
  const merged = [...existing];
  for (const revision of incoming) {
    const index = merged.findIndex((current) => current.version === revision.version);
    if (index < 0) merged.push(revision);
    else merged[index] = revision;
  }
  return merged.sort((left, right) => right.version - left.version);
}

export function groupActivitiesByOccurrenceDay(items: readonly ActivityDetail[]): ActivityDayGroup[] {
  const grouped: Record<string, ActivityDetail[]> = {};
  for (const detail of newestActivitiesFirst(items)) {
    const localDate = manualActivityParticipantNow(
      detail.activity.startedAt,
      detail.activity.occurrenceTimeZone,
    ).localDate;
    const day = grouped[localDate] ?? [];
    day.push(detail);
    grouped[localDate] = day;
  }
  return Object.entries(grouped)
    .sort(([left], [right]) => right.localeCompare(left))
    .map(([localDate, dayItems]) => ({ localDate, items: dayItems }));
}

export function activityBelongsToProfile(detail: ActivityDetail, profileID: string): boolean {
  return detail.activity.participantId === profileID;
}

export function activityWasEdited(detail: ActivityDetail): boolean {
  return detail.version > 1;
}

export function priorNoteForProfile(
  revision: ActivityRevision,
  profileID: string,
): string | null {
  return revision.participantId === profileID && revision.note ? revision.note : null;
}

export function activityEditSeed(detail: ActivityDetail, defaults: ManualActivityDefaults): {
  defaults: ManualActivityDefaults;
  form: ManualActivityFormState;
  note: string;
} {
  const occurrenceTimeZone = detail.activity.occurrenceTimeZone;
  const retainedStart = manualActivityParticipantNow(detail.activity.startedAt, occurrenceTimeZone);
  const participantNow = manualActivityParticipantNow(defaults.currentInstant, occurrenceTimeZone);
  return {
    defaults: {
      ...defaults,
      timeZone: occurrenceTimeZone,
      localDate: participantNow.localDate,
      localStartTime: participantNow.localTime,
    },
    form: {
      localDate: retainedStart.localDate,
      localTime: retainedStart.localTime,
      durationSeconds: String(detail.activity.durationSeconds),
      occurrenceTouched: true,
    },
    note: detail.activity.note ?? '',
  };
}
