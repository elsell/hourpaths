import { createSessionApiClient, generatedResponse, type GeneratedOperationResult, type OnboardingProfile, type SessionPath, type TimerState } from '@hourpaths/api-client';
import { authenticatedProfileFromAPI, mergePendingInvitationPage, pathArchiveLists, pathsRequiringTimerRestore, sessionFailureFromResponse, validateSessionCredential, type PendingPathInvitation, type PendingPathInvitationState, type SessionExchangeCredential } from '@hourpaths/client-core';
import { homePreferencesFromAPI, type HomePreferenceSnapshot } from './home-preference-operation';
import type { HomePath } from './ui/home-organization';

type PendingInvitationEnvelope = {
  data: PendingPathInvitation[];
  meta: { nextCursor?: string };
};

function pendingInvitationResponse(result: GeneratedOperationResult<unknown>) {
  const response = generatedResponse(result);
  return {
    ...response,
    json: async () => {
      const envelope = await response.json() as PendingInvitationEnvelope | undefined;
      return envelope ? {
        data: {
          items: envelope.data,
          nextCursor: envelope.meta.nextCursor ?? '',
        },
      } : undefined;
    },
  };
}

export type MobileHomeProfile = {
  id: string;
  email: string;
  displayName: string;
  profileVisibility: 'public' | 'private';
  homePreferences: HomePreferenceSnapshot;
  paths: HomePath[];
  archivedPaths: SessionPath[];
  pendingInvitations: PendingPathInvitationState;
  timers: Record<string, TimerState>;
};
export type MobileOnboardingProfile = OnboardingProfile;
export type MobileOnboardingDraft = MobileOnboardingProfile & {
  profileVisibility: 'public' | 'private' | null;
  timeZone: string | null;
  firstDayOfWeek: number | null;
  usernameReviewed: boolean;
  atLeast16: boolean;
  termsAccepted: boolean;
  privacyAcknowledged: boolean;
  communityGuidelinesAccepted: boolean;
};

export function applyRefreshedMobilePath(
  profile: MobileHomeProfile,
  pathID: string,
  refreshed: HomePath | null,
): MobileHomeProfile {
  const replaceOrAppend = <T extends SessionPath>(paths: T[], path: T): T[] => {
    const found = paths.some(({ id }) => id === pathID);
    return found
      ? paths.map((candidate) => candidate.id === pathID ? path : candidate)
      : [...paths, path];
  };
  const active = refreshed && !refreshed.archivedAt
    ? replaceOrAppend(profile.paths, refreshed)
    : profile.paths.filter(({ id }) => id !== pathID);
  const archived = refreshed?.archivedAt
    ? replaceOrAppend(profile.archivedPaths, refreshed)
    : profile.archivedPaths.filter(({ id }) => id !== pathID);
  let timers = profile.timers;
  if (!refreshed || refreshed.archivedAt) {
    const { [pathID]: _removed, ...retained } = timers;
    timers = retained;
  }
  return { ...profile, archivedPaths: archived, paths: active, timers };
}

export function expoCalendarWeekdayToISO(firstWeekday: number | null | undefined): number | null {
  return firstWeekday && firstWeekday >= 1 && firstWeekday <= 7
    ? ((firstWeekday + 5) % 7) + 1
    : null;
}

export function createMobileOnboardingDraft(
  profile: MobileOnboardingProfile,
  device: { timeZone: string | null; firstDayOfWeek: number | null },
): MobileOnboardingDraft {
  return {
    ...profile,
    profileVisibility: null,
    timeZone: device.timeZone,
    firstDayOfWeek: device.firstDayOfWeek,
    usernameReviewed: false,
    atLeast16: false,
    termsAccepted: false,
    privacyAcknowledged: false,
    communityGuidelinesAccepted: false,
  };
}

export function refreshMobileOnboardingPolicy(
  draft: MobileOnboardingDraft,
  current: MobileOnboardingProfile,
): MobileOnboardingDraft {
  return {
    ...draft,
    email: current.email,
    policies: current.policies,
    policyReviewToken: current.policyReviewToken,
    termsAccepted: false,
    privacyAcknowledged: false,
    communityGuidelinesAccepted: false,
  };
}

export function canCompleteMobileOnboarding(draft: MobileOnboardingDraft): boolean {
  return Boolean(
    validProfileUsername(draft.usernameSuggestion) && validProfileDisplayName(draft.displayName) &&
    draft.usernameReviewed && draft.profileVisibility &&
    draft.timeZone && draft.firstDayOfWeek && draft.atLeast16 && draft.termsAccepted &&
    draft.privacyAcknowledged && draft.communityGuidelinesAccepted,
  );
}

export function validProfileDisplayName(displayName: string): boolean {
  return displayName.trim().length > 0;
}

export function validProfileUsername(username: string): boolean {
  return username.length >= 3 && username.length <= 64 && /^[A-Za-z0-9._]+$/.test(username);
}

export function replaceMobileOnboardingUsername<T extends { usernameSuggestion: string; usernameReviewed: boolean }>(
  profile: T,
  usernameSuggestion: string,
): T {
  return { ...profile, usernameSuggestion, usernameReviewed: false };
}

const pathCapabilityKeys = [
  'inviteMembers', 'leavePath', 'manageGoals', 'manageLifecycle', 'manageMembers',
  'manageVisibility', 'renamePath', 'trackTime', 'transferOwnership',
] as const;

function exactObjectKeys(value: Record<string, unknown>, expected: string[]): boolean {
  const keys = Object.keys(value).sort();
  return keys.length === expected.length && keys.every((key, index) => key === expected[index]);
}

function positiveSeconds(value: unknown): value is number {
  return Number.isSafeInteger(value) && (value as number) > 0;
}

function validIntervalGoal(value: unknown): boolean {
  if (!value || typeof value !== 'object') return false;
  const goal = value as Record<string, unknown>;
  if (!exactObjectKeys(goal, ['alignment', 'recurrence', 'targetSeconds'])
    || !positiveSeconds(goal.targetSeconds)
    || !['hourly', 'daily', 'weekly', 'monthly', 'yearly'].includes(String(goal.recurrence))
    || !goal.alignment || typeof goal.alignment !== 'object') return false;
  const alignment = goal.alignment as Record<string, unknown>;
  const integerBetween = (candidate: unknown, minimum: number, maximum: number) =>
    Number.isSafeInteger(candidate) && (candidate as number) >= minimum && (candidate as number) <= maximum;
  switch (goal.recurrence) {
    case 'hourly': return exactObjectKeys(alignment, ['minute']) && integerBetween(alignment.minute, 0, 59);
    case 'daily': return exactObjectKeys(alignment, ['hour']) && integerBetween(alignment.hour, 0, 23);
    case 'weekly': return exactObjectKeys(alignment, ['isoWeekday']) && integerBetween(alignment.isoWeekday, 1, 7);
    case 'monthly': return exactObjectKeys(alignment, ['day']) && integerBetween(alignment.day, 1, 31);
    case 'yearly': {
      if (!exactObjectKeys(alignment, ['day', 'month'])
        || !integerBetween(alignment.month, 1, 12)
        || !integerBetween(alignment.day, 1, 31)) return false;
      const candidate = new Date(Date.UTC(2000, alignment.month as number - 1, alignment.day as number));
      return candidate.getUTCMonth() === alignment.month as number - 1
        && candidate.getUTCDate() === alignment.day;
    }
    default: return false;
  }
}

function validOverallTarget(value: unknown): boolean {
  if (!value || typeof value !== 'object') return false;
  const target = value as Record<string, unknown>;
  return exactObjectKeys(target, ['targetSeconds']) && positiveSeconds(target.targetSeconds);
}

function validRFC3339(value: unknown): value is string {
  if (typeof value !== 'string') return false;
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-](\d{2}):(\d{2}))$/.exec(value);
  if (!match) return false;
  const [, year, month, day, hour, minute, second, offsetHour, offsetMinute] = match;
  const candidate = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
  return candidate.getUTCFullYear() === Number(year)
    && candidate.getUTCMonth() === Number(month) - 1
    && candidate.getUTCDate() === Number(day)
    && Number(hour) <= 23 && Number(minute) <= 59 && Number(second) <= 59
    && (offsetHour === undefined || (Number(offsetHour) <= 23 && Number(offsetMinute) <= 59));
}

function validHomeOrganization(value: unknown): value is HomePath['home'] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const home = value as Record<string, unknown>;
  const keys = Object.keys(home).sort();
  const allowed = ['classification', 'manualPosition', 'pinned', 'pinnedPosition', 'recentActivityAt'];
  if (keys.some((key) => !allowed.includes(key)) || !keys.includes('classification') || !keys.includes('pinned')) return false;
  const position = (candidate: unknown) => Number.isSafeInteger(candidate) && (candidate as number) >= 0;
  return ['solo', 'shared', 'supporting'].includes(String(home.classification))
    && typeof home.pinned === 'boolean'
    && (home.pinnedPosition === undefined || position(home.pinnedPosition))
    && (home.manualPosition === undefined || position(home.manualPosition))
    && (home.recentActivityAt === undefined || validRFC3339(home.recentActivityAt))
    && home.pinned === (home.pinnedPosition !== undefined)
    && !(home.classification === 'supporting' && home.recentActivityAt !== undefined);
}

function isCompleteSessionPath(value: unknown): value is HomePath {
  if (!value || typeof value !== 'object') return false;
  const path = value as Record<string, unknown>;
  const capabilities = path.capabilities;
  return typeof path.id === 'string' && Boolean(path.id.trim())
    && typeof path.name === 'string' && Boolean(path.name.trim())
    && ['private', 'followers', 'public'].includes(String(path.visibility))
    && Boolean(capabilities) && typeof capabilities === 'object'
    && pathCapabilityKeys.every((key) => typeof (capabilities as Record<string, unknown>)[key] === 'boolean')
    && (path.archivedAt === undefined || validRFC3339(path.archivedAt))
    && (path.intervalGoal === undefined || validIntervalGoal(path.intervalGoal))
    && (path.overallTarget === undefined || validOverallTarget(path.overallTarget))
    && validHomeOrganization(path.home)
    && (path.archivedAt !== undefined || ((path.home as HomePath['home']).classification === 'supporting'
      ? (capabilities as Record<string, unknown>).trackTime === false
      : (capabilities as Record<string, unknown>).trackTime === true));
}

export function admitMobileSessionPaths(active: unknown, archived: unknown): {
  active: HomePath[];
  archived: HomePath[];
} {
  if (!Array.isArray(active) || !Array.isArray(archived)
    || active.some((path) => !isCompleteSessionPath(path))
    || archived.some((path) => !isCompleteSessionPath(path))) throw sessionFailureFromResponse(502);
  const identifiers = [...active, ...archived].map((path) => path.id);
  if (new Set(identifiers).size !== identifiers.length) throw sessionFailureFromResponse(502);
  return { active, archived };
}

type HomePathPage = { paths: unknown; homePreferences: HomePreferenceSnapshot; nextCursor: string };

function homePathPageResponse(result: GeneratedOperationResult<unknown>) {
  const response = generatedResponse(result);
  return {
    ...response,
    json: async () => {
      const envelope = await response.json() as { data?: unknown; meta?: { homePreferences?: unknown; nextCursor?: unknown } } | undefined;
      if (!envelope?.meta?.homePreferences) return undefined;
      return { data: {
        paths: envelope.data,
        homePreferences: homePreferencesFromAPI(envelope.meta.homePreferences),
        nextCursor: typeof envelope.meta.nextCursor === 'string' ? envelope.meta.nextCursor : '',
      } };
    },
  };
}

async function loadAllHomePathPages(
  apiURL: string,
  credential: SessionExchangeCredential,
  archived: boolean,
): Promise<{ paths: unknown[]; homePreferences: HomePreferenceSnapshot }> {
  const paths: unknown[] = [];
  const seenCursors = new Set<string>();
  let cursor: string | undefined;
  let homePreferences: HomePreferenceSnapshot | undefined;
  do {
    const page = await validateSessionCredential<HomePathPage>(credential, async (current) => homePathPageResponse(
      archived
        ? await createSessionApiClient(apiURL, () => current.token).archivedPaths(cursor)
        : await createSessionApiClient(apiURL, () => current.token).paths(cursor),
    ));
    if (!Array.isArray(page.paths)) throw sessionFailureFromResponse(502);
    if (homePreferences && JSON.stringify(homePreferences) !== JSON.stringify(page.homePreferences)) {
      throw sessionFailureFromResponse(409);
    }
    paths.push(...page.paths);
    homePreferences ??= page.homePreferences;
    cursor = page.nextCursor || undefined;
    if (cursor && seenCursors.has(cursor)) throw sessionFailureFromResponse(502);
    if (cursor) seenCursors.add(cursor);
  } while (cursor);
  if (!homePreferences) throw sessionFailureFromResponse(502);
  return { paths, homePreferences };
}

export async function loadMobileHomeProfile(
  apiURL: string,
  credential: SessionExchangeCredential,
): Promise<MobileHomeProfile> {
  const [profile, pathsResponse, archivedPathsResponse, pendingInvitations] = await Promise.all([
    validateSessionCredential<unknown>(credential, async (current) => generatedResponse(
      await createSessionApiClient(apiURL, () => current.token).profile(),
    )).then((value) => {
      try {
        return authenticatedProfileFromAPI(value);
      } catch {
        throw sessionFailureFromResponse(502);
      }
    }),
    loadAllHomePathPages(apiURL, credential, false),
    loadAllHomePathPages(apiURL, credential, true),
    validateSessionCredential<PendingPathInvitationState>(credential, async (current) => pendingInvitationResponse(
      await createSessionApiClient(apiURL, () => current.token).pendingPathInvitations(),
    )),
  ]);
  if (JSON.stringify(pathsResponse.homePreferences) !== JSON.stringify(archivedPathsResponse.homePreferences)) {
    throw sessionFailureFromResponse(409);
  }
  const admitted = admitMobileSessionPaths(pathsResponse.paths, archivedPathsResponse.paths);
  const paths = admitted.active;
  const archivedPaths = admitted.archived;
  const partitioned = pathArchiveLists([...paths, ...archivedPaths]);
  const trackablePaths = pathsRequiringTimerRestore(partitioned.activePaths);
  const states = await Promise.all(trackablePaths.map((path) =>
    validateSessionCredential<TimerState>(credential, async (current) => generatedResponse(
      await createSessionApiClient(apiURL, () => current.token).currentTimer(path.id),
    )),
  ));
  return {
    ...profile,
    homePreferences: pathsResponse.homePreferences,
    paths: partitioned.activePaths,
    archivedPaths: partitioned.archivedPaths,
    pendingInvitations: mergePendingInvitationPage(
      { items: [], nextCursor: '' },
      pendingInvitations,
      '',
    ),
    timers: Object.fromEntries(trackablePaths.map((path, index) => [path.id, states[index]!])),
  };
}

export function loadMobileOnboardingProfile(
  apiURL: string,
  credential: SessionExchangeCredential,
): Promise<MobileOnboardingProfile> {
  return validateSessionCredential<MobileOnboardingProfile>(credential, async (current) => generatedResponse(
    await createSessionApiClient(apiURL, () => current.token).onboarding(),
  ));
}
