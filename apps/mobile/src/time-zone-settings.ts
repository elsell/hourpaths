export type TimeZonePreference = Readonly<{
  changed: boolean;
  effectiveAt: string;
  timeZone: string;
}>;

export type FrozenTimeZoneChangeIntent = Readonly<{
  confirmed: true;
  idempotencyKey: string;
  proposedTimeZone: string;
  reviewedTimeZone: string;
}>;

function isIANATimeZone(value: string) {
  if (!value || value.trim() !== value || value === 'Local') return false;
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: value }).format(0);
    return true;
  } catch {
    return false;
  }
}

export function timeZonePreferenceFromAPI(value: unknown): TimeZonePreference {
  if (!value || typeof value !== 'object') throw new Error('invalid_time_zone_preference');
  const candidate = value as Record<string, unknown>;
  if (
    Object.keys(candidate).sort().join(',') !== 'changed,effectiveAt,timeZone' ||
    typeof candidate.changed !== 'boolean' ||
    typeof candidate.effectiveAt !== 'string' ||
    typeof candidate.timeZone !== 'string' ||
    !isIANATimeZone(candidate.timeZone)
  ) throw new Error('invalid_time_zone_preference');
  const effectiveAt = new Date(candidate.effectiveAt);
  if (!Number.isFinite(effectiveAt.getTime())) throw new Error('invalid_time_zone_preference');
  return { changed: candidate.changed, effectiveAt: effectiveAt.toISOString(), timeZone: candidate.timeZone };
}

export function filterTimeZones(
  available: readonly string[],
  query: string,
  configuredTimeZone: string,
) {
  const normalizedQuery = query.trim().toLowerCase();
  const unique = new Set(available.filter(isIANATimeZone));
  if (isIANATimeZone(configuredTimeZone)) unique.add(configuredTimeZone);
  return [...unique]
    .filter((timeZone) => !normalizedQuery || timeZone.toLowerCase().includes(normalizedQuery))
    .sort((left, right) => {
      if (!normalizedQuery && left === configuredTimeZone) return -1;
      if (!normalizedQuery && right === configuredTimeZone) return 1;
      return left.localeCompare(right);
    });
}

export function createTimeZoneChangeIntentCoordinator(createKey: () => string) {
  let frozen: FrozenTimeZoneChangeIntent | undefined;
  return {
    complete(intent: FrozenTimeZoneChangeIntent) {
      if (frozen === intent) frozen = undefined;
    },
    freeze(reviewedTimeZone: string, proposedTimeZone: string): FrozenTimeZoneChangeIntent {
      if (!isIANATimeZone(reviewedTimeZone) || !isIANATimeZone(proposedTimeZone)) {
        throw new Error('invalid_time_zone_change');
      }
      if (
        frozen &&
        frozen.reviewedTimeZone === reviewedTimeZone &&
        frozen.proposedTimeZone === proposedTimeZone
      ) return frozen;
      frozen = { confirmed: true, idempotencyKey: createKey(), proposedTimeZone, reviewedTimeZone };
      return frozen;
    },
    invalidate() {
      frozen = undefined;
    },
  };
}
