import type { SessionPath, TimerState } from '@hourpaths/api-client';
import { effectivePathCapabilities } from '@hourpaths/client-core';
import { homePathSections } from './home-path-sections';

export const homeFilters = ['all', 'solo', 'shared', 'supporting'] as const;
export type HomeFilter = typeof homeFilters[number];
export const homeOrders = ['recent', 'alphabetical', 'manual'] as const;
export type HomeOrder = typeof homeOrders[number];

export type HomePath = SessionPath & {
  home: {
    classification: 'solo' | 'shared' | 'supporting';
    pinned: boolean;
    pinnedPosition?: number;
    manualPosition?: number;
    recentActivityAt?: string;
  };
};

export type HomePreferences = {
  order: HomeOrder;
  pinnedPathIDs: string[];
  manualPathIDs: string[];
};

export type OrganizedHomePaths<P extends HomePath = HomePath> = {
  active: P[];
  pinned: P[];
  trackable: P[];
  supporting: P[];
};

export function defaultHomePreferences(): HomePreferences {
  return { order: 'recent', pinnedPathIDs: [], manualPathIDs: [] };
}

function alphabetical<P extends HomePath>(left: P, right: P): number {
  return left.name.localeCompare(right.name, undefined, { sensitivity: 'base' }) || left.id.localeCompare(right.id);
}

function ordered<P extends HomePath>(paths: readonly P[], preferences: HomePreferences): P[] {
  const sourceIndex = new Map(paths.map((path, index) => [path.id, index]));
  const manualIndex = new Map(preferences.manualPathIDs.map((id, index) => [id, index]));
  return [...paths].sort((left, right) => {
    if (preferences.order === 'manual') {
      const leftIndex = manualIndex.get(left.id);
      const rightIndex = manualIndex.get(right.id);
      if (leftIndex !== undefined || rightIndex !== undefined) {
        if (leftIndex === undefined) return 1;
        if (rightIndex === undefined) return -1;
        return leftIndex - rightIndex;
      }
    }
    if (preferences.order === 'recent') {
      const leftInstant = left.home.recentActivityAt ? Date.parse(left.home.recentActivityAt) : Number.NaN;
      const rightInstant = right.home.recentActivityAt ? Date.parse(right.home.recentActivityAt) : Number.NaN;
      const leftValid = Number.isFinite(leftInstant);
      const rightValid = Number.isFinite(rightInstant);
      if (leftValid !== rightValid) return leftValid ? -1 : 1;
      if (leftValid && rightValid && leftInstant !== rightInstant) return rightInstant - leftInstant;
    }
    const byName = alphabetical(left, right);
    return byName || (sourceIndex.get(left.id) ?? 0) - (sourceIndex.get(right.id) ?? 0);
  });
}

function matchesFilter(path: HomePath, filter: HomeFilter): boolean {
  if (filter === 'all') return true;
  if (filter === 'supporting') return path.home.classification === 'supporting';
  if (path.home.classification === 'supporting' || !effectivePathCapabilities(path).trackTime) return false;
  return path.home.classification === filter;
}

export function organizeHomePaths<P extends HomePath>(
  paths: readonly P[],
  timers: Readonly<Record<string, TimerState | undefined>>,
  preferences: HomePreferences,
  filter: HomeFilter,
): OrganizedHomePaths<P> {
  const visible = paths.filter((path) => matchesFilter(path, filter));
  const trackable = visible.filter((path) => path.home.classification !== 'supporting' && effectivePathCapabilities(path).trackTime);
  const supporting = visible.filter((path) => path.home.classification === 'supporting');
  const sections = homePathSections(trackable, timers);
  const activeIDs = new Set(sections.active.map(({ id }) => id));
  const pinnedIDs = new Set(preferences.pinnedPathIDs);
  const ordinary = sections.ordinary.filter(({ id }) => !activeIDs.has(id));
  const pinned = ordered(ordinary.filter(({ id }) => pinnedIDs.has(id)), {
    ...preferences,
    order: 'manual',
    manualPathIDs: preferences.pinnedPathIDs,
  });
  const unpinned = ordered(ordinary.filter(({ id }) => !pinnedIDs.has(id)), preferences);
  const pinnedSupporting = ordered(supporting.filter(({ id }) => pinnedIDs.has(id)), {
    ...preferences,
    order: 'manual',
    manualPathIDs: preferences.pinnedPathIDs,
  });
  const unpinnedSupporting = ordered(supporting.filter(({ id }) => !pinnedIDs.has(id)), preferences);
  const orderedSupporting = [...pinnedSupporting, ...unpinnedSupporting];
  return { active: sections.active, pinned, trackable: unpinned, supporting: orderedSupporting };
}

export function pinHomePath(preferences: HomePreferences, pathID: string): HomePreferences {
  if (!pathID || preferences.pinnedPathIDs.includes(pathID)) return preferences;
  return { ...preferences, pinnedPathIDs: [...preferences.pinnedPathIDs, pathID] };
}

export function unpinHomePath(preferences: HomePreferences, pathID: string): HomePreferences {
  if (!preferences.pinnedPathIDs.includes(pathID)) return preferences;
  return { ...preferences, pinnedPathIDs: preferences.pinnedPathIDs.filter((id) => id !== pathID) };
}

export function moveHomePath(preferences: HomePreferences, pathID: string, offset: -1 | 1): HomePreferences {
  const key = preferences.pinnedPathIDs.includes(pathID) ? 'pinnedPathIDs' : 'manualPathIDs';
  const values = preferences[key];
  const from = values.indexOf(pathID);
  const to = from + offset;
  if (from < 0 || to < 0 || to >= values.length) return preferences;
  const moved = [...values];
  [moved[from], moved[to]] = [moved[to]!, moved[from]!];
  return { ...preferences, [key]: moved };
}

export function reorderHomePaths(
  preferences: HomePreferences,
  collection: 'manual' | 'pinned',
  sourceIndices: readonly number[],
  destination: number,
): HomePreferences {
  const key = collection === 'pinned' ? 'pinnedPathIDs' : 'manualPathIDs';
  const values = preferences[key];
  const sources = [...new Set(sourceIndices)].sort((left, right) => left - right);
  if (sources.length === 0 || sources.some((index) => !Number.isSafeInteger(index) || index < 0 || index >= values.length)
    || !Number.isSafeInteger(destination) || destination < 0 || destination > values.length) return preferences;
  const selected = new Set(sources);
  const moving = sources.map((index) => values[index]!);
  const remaining = values.filter((_, index) => !selected.has(index));
  const removedBeforeDestination = sources.filter((index) => index < destination).length;
  const insertion = Math.max(0, Math.min(remaining.length, destination - removedBeforeDestination));
  const reordered = [...remaining.slice(0, insertion), ...moving, ...remaining.slice(insertion)];
  if (reordered.every((id, index) => id === values[index])) return preferences;
  return { ...preferences, [key]: reordered };
}

export function reorderVisibleHomePaths(
  preferences: HomePreferences,
  collection: 'manual' | 'pinned',
  visiblePathIDs: readonly string[],
  sourceIndices: readonly number[],
  destination: number,
): HomePreferences {
  const key = collection === 'pinned' ? 'pinnedPathIDs' : 'manualPathIDs';
  const visible = new Set(visiblePathIDs);
  const currentVisible = preferences[key].filter((id) => visible.has(id));
  const temporary: HomePreferences = { ...preferences, [key]: currentVisible };
  const moved = reorderHomePaths(temporary, collection, sourceIndices, destination);
  if (moved === temporary) return preferences;
  let nextVisible = 0;
  const merged = preferences[key].map((id) => visible.has(id) ? moved[key][nextVisible++]! : id);
  return { ...preferences, [key]: merged };
}

function exactKeys(value: Record<string, unknown>, expected: readonly string[]): boolean {
  const keys = Object.keys(value).sort();
  const sortedExpected = [...expected].sort();
  return keys.length === sortedExpected.length && keys.every((key, index) => key === sortedExpected[index]);
}

function validIDs(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((id) => typeof id === 'string' && id.length > 0) && new Set(value).size === value.length;
}

function validPreferences(value: unknown): value is HomePreferences {
  if (!value || typeof value !== 'object' || !exactKeys(value as Record<string, unknown>, ['manualPathIDs', 'order', 'pinnedPathIDs'])) return false;
  const candidate = value as Record<string, unknown>;
  return homeOrders.includes(candidate.order as HomeOrder) && validIDs(candidate.pinnedPathIDs) && validIDs(candidate.manualPathIDs);
}

export function serializeHomePreferences(ownerID: string, preferences: HomePreferences): string {
  if (!ownerID || !validPreferences(preferences)) throw new Error('invalid Home preferences');
  return JSON.stringify({ ownerID, preferences, version: 1 });
}

export function parseHomePreferences(value: string | null, ownerID: string): HomePreferences | null {
  if (!value || !ownerID) return null;
  try {
    const parsed = JSON.parse(value) as unknown;
    if (!parsed || typeof parsed !== 'object' || !exactKeys(parsed as Record<string, unknown>, ['ownerID', 'preferences', 'version'])) return null;
    const envelope = parsed as Record<string, unknown>;
    return envelope.version === 1 && envelope.ownerID === ownerID && validPreferences(envelope.preferences)
      ? envelope.preferences
      : null;
  } catch {
    return null;
  }
}
