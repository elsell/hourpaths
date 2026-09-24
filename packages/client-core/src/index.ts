export {
  authenticatedProfileFromAPI,
  type AuthenticatedProfile,
} from './authenticated-profile';
export {
  applyPathLeaveResult,
  createPathLeaveOperationOwner,
  reviewPathLeave,
  type PathLeaveMutationResult,
  type PathLeavePath,
  type PathLeaveReceipt,
  type PathLeaveRequestBody,
  type PathLeaveReview,
} from './path-leave';
export {
  applyActivityDeletionResult,
  createActivityDeletionOperationOwner,
  type ActivityDeletionIdentity,
  type ActivityDeletionMutationResult,
  type ActivityDeletionResult,
} from './activity-deletion';

export {
  createSignOutTimerResolutionCoordinator,
  type RunningTimerSnapshot,
  type SignOutTimerFailure,
  type SignOutTimerResolution,
  type SignOutTimerResolutionCoordinator,
  type SignOutTimerResolutionState,
  type StopAndSaveTimer,
  type TimerStopAcknowledgement,
} from './sign-out-timer-resolution';

export {
  createNotificationRefreshLatch,
  type NotificationRefreshLatch,
} from './notification-refresh';

export {
  createForegroundNotificationCoordinator,
  type ForegroundNotificationContext,
  type ForegroundNotificationCoordinator,
  type ForegroundNotificationCoordinatorPorts,
  type ForegroundNotificationOutcome,
  type ForegroundNotificationPresentation,
  type ForegroundNotificationReceipt,
  type ForegroundNotificationResolution,
} from './foreground-notifications';

export function activeTimerSeconds(startedAt: string | undefined, now: number): number {
  if (!startedAt || !Number.isFinite(now)) return 0;
  const startedAtMilliseconds = Date.parse(startedAt);
  if (!Number.isFinite(startedAtMilliseconds)) return 0;
  return Math.max(0, Math.floor((now - startedAtMilliseconds) / 1000));
}

export type OverallTarget = { targetSeconds: number };

export type OverallProgress = {
  accumulatedSeconds: number;
  targetSeconds: number;
  visualSeconds: number;
  completed: boolean;
};

export function overallProgress(
  accumulatedSeconds: number,
  overallTarget?: OverallTarget,
): OverallProgress | undefined {
  if (!overallTarget) return undefined;
  return {
    accumulatedSeconds,
    targetSeconds: overallTarget.targetSeconds,
    visualSeconds: Math.min(accumulatedSeconds, overallTarget.targetSeconds),
    completed: accumulatedSeconds >= overallTarget.targetSeconds,
  };
}

export type IntervalProgressProjection = {
  accumulatedSeconds: number;
  targetSeconds: number;
};

export type IntervalProgress = IntervalProgressProjection & {
  visualSeconds: number;
  completed: boolean;
};

export function intervalProgress(projection?: IntervalProgressProjection): IntervalProgress | undefined {
  if (!projection) return undefined;
  return {
    ...projection,
    visualSeconds: Math.min(projection.accumulatedSeconds, projection.targetSeconds),
    completed: projection.accumulatedSeconds >= projection.targetSeconds,
  };
}

export type GoalRecurrence = 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly';

export type GoalAlignment = {
  minute?: number;
  hour?: number;
  isoWeekday?: number;
  month?: number;
  day?: number;
};

export type GoalConfiguration = {
  intervalGoal?: {
    targetSeconds: number;
    recurrence: GoalRecurrence;
    alignment?: GoalAlignment;
  };
  overallTarget?: OverallTarget;
};

export type GoalConfigurationComparison = {
  current: GoalConfiguration;
  proposed: GoalConfiguration;
  changed: boolean;
};

function copyGoalConfiguration(configuration: GoalConfiguration): GoalConfiguration {
  const alignment = configuration.intervalGoal?.alignment;
  const retainedAlignment = alignment ? {
    ...(alignment.minute !== undefined ? { minute: alignment.minute } : {}),
    ...(alignment.hour !== undefined ? { hour: alignment.hour } : {}),
    ...(alignment.isoWeekday !== undefined ? { isoWeekday: alignment.isoWeekday } : {}),
    ...(alignment.month !== undefined ? { month: alignment.month } : {}),
    ...(alignment.day !== undefined ? { day: alignment.day } : {}),
  } : undefined;
  return {
    ...(configuration.intervalGoal ? {
      intervalGoal: {
        targetSeconds: configuration.intervalGoal.targetSeconds,
        recurrence: configuration.intervalGoal.recurrence,
        ...(retainedAlignment ? {
          alignment: retainedAlignment,
        } : {}),
      },
    } : {}),
    ...(configuration.overallTarget ? {
      overallTarget: { targetSeconds: configuration.overallTarget.targetSeconds },
    } : {}),
  };
}

export function compareGoalConfigurations(
  current: GoalConfiguration,
  proposed: GoalConfiguration,
): GoalConfigurationComparison {
  const retainedCurrent = copyGoalConfiguration(current);
  const retainedProposed = copyGoalConfiguration(proposed);
  return {
    current: retainedCurrent,
    proposed: retainedProposed,
    changed: JSON.stringify(retainedCurrent) !== JSON.stringify(retainedProposed),
  };
}

export type GoalMutationPath = GoalConfiguration & CapabilityPath & {
  [key: string]: unknown;
};

export type GoalMutationTimerState = {
  running: boolean;
  accumulatedSeconds: number;
  intervalProgress?: IntervalProgressProjection;
  [key: string]: unknown;
};

export type GoalMutationResult = {
  path: GoalMutationPath;
  accumulatedSeconds: number;
  intervalProgress?: IntervalProgressProjection;
};

export function applyGoalMutationResult<
  P extends GoalMutationPath,
  T extends GoalMutationTimerState,
  S extends {
    paths: P[];
    selectedPath?: P | null;
    timerStates: Record<string, T>;
  },
>(
  state: S,
  result: GoalMutationResult,
): Omit<S, 'paths' | 'selectedPath' | 'timerStates'> & {
  paths: Array<P | GoalMutationPath>;
  selectedPath: P | GoalMutationPath | null | undefined;
  timerStates: Record<string, T | GoalMutationTimerState>;
} {
  const currentTimer = state.timerStates[result.path.id];
  return {
    ...state,
    paths: state.paths.map((path) => path.id === result.path.id ? result.path : path),
    selectedPath: state.selectedPath?.id === result.path.id ? result.path : state.selectedPath,
    timerStates: currentTimer ? {
      ...state.timerStates,
      [result.path.id]: {
        ...currentTimer,
        accumulatedSeconds: result.accumulatedSeconds,
        intervalProgress: result.intervalProgress,
      },
    } : state.timerStates,
  };
}

export type SessionAccessState =
  | 'authenticated_online'
  | 'authenticated_offline'
  | 'authentication_required'
  | 'local_session_unreadable';

export const knownProblemCodes = [
  'bad_request',
  'validation_failed',
  'unauthenticated',
  'invalid_credential',
  'forbidden',
  'not_found',
  'conflict',
  'idempotency_conflict',
  'rate_limited',
  'authorization_pending',
  'authorization_dead_lettered',
  'authorization_policy_not_configured',
  'unavailable',
  'internal_error',
  'request_failed',
  'invalid_identity_token',
  'identity_forbidden',
  'identity_exchange_unavailable',
] as const;
export type ProblemCode = (typeof knownProblemCodes)[number];

export type SessionFailure =
  | { kind: 'network' }
  | { kind: 'http'; status: number; code?: ProblemCode }
  | { kind: 'expired' }
  | { kind: 'local_storage'; reason: 'missing_fields' | 'malformed' };

export type SessionFailureDecision = {
  state: SessionAccessState;
  discardCredential: boolean;
  retryable: boolean;
};

export const sessionNextActions = ['home', 'onboarding', 'duplicate_email_recovery'] as const;
export type SessionNextAction = (typeof sessionNextActions)[number];
export type SessionCredential = { token: string; expiresAt: string; nextAction?: SessionNextAction };
export type SessionExchangeCredential = SessionCredential & { nextAction: SessionNextAction };
export type SessionRefreshResponse = { ok: boolean; status: number; problem?: unknown; json(): Promise<unknown> };
export type ClientRuntimeConfig = {
  environment: 'development' | 'production';
  apiURL: string;
  oidcIssuer: string;
  oidcClientId: string;
};

export type ManualActivityLocalDateTime = {
  localDate: string;
  localTime: string;
};
export type ManualActivityParticipantNow = ManualActivityLocalDateTime & {
  currentInstant: string;
  timeZone: string;
};
export type ManualActivityFormState = ManualActivityLocalDateTime & {
  durationSeconds: string;
  occurrenceTouched: boolean;
};
export type ManualActivityFields = ManualActivityLocalDateTime & {
  durationSeconds: number;
};
export type ManualActivityFormResult =
  | { ok: true; fields: ManualActivityFields }
  | { ok: false; reason: 'duration' | 'occurrence' | 'future_end' };

export type SessionOperationTicket = { current(): boolean };
export type SessionOperationOwner = {
  issue(): SessionOperationTicket;
  invalidate(): void;
};

const localDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const localTimePattern = /^(\d{2}):(\d{2}):(\d{2})$/;

function localDateTimeValue(value: ManualActivityLocalDateTime): number | undefined {
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

function participantLocalDateTime(value: number, timeZone: string): ManualActivityLocalDateTime | undefined {
  if (!Number.isFinite(value) || !timeZone || timeZone === 'Local' || timeZone.trim() !== timeZone) return undefined;
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
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
    }).formatToParts(new Date(value));
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

function participantInstantValue(value: ManualActivityLocalDateTime, timeZone: string): number | undefined {
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

function positiveWholeSeconds(value: string): number | undefined {
  if (!/^[1-9]\d*$/.test(value)) return undefined;
  const seconds = Number(value);
  return Number.isSafeInteger(seconds) ? seconds : undefined;
}

export function manualActivityParticipantNow(
  currentInstant: string,
  timeZone: string,
  elapsedMilliseconds = 0,
): ManualActivityParticipantNow {
  const base = Date.parse(currentInstant);
  if (!Number.isFinite(base) || !Number.isFinite(elapsedMilliseconds) || elapsedMilliseconds < 0) {
    throw new Error('participant current instant');
  }
  const instant = base + elapsedMilliseconds;
  const local = participantLocalDateTime(instant, timeZone);
  if (!local) throw new Error('participant time zone');
  return { ...local, currentInstant: new Date(instant).toISOString(), timeZone };
}

export function createManualActivityFormState(now: ManualActivityParticipantNow): ManualActivityFormState {
  if (localDateTimeValue(now) === undefined || !Number.isFinite(Date.parse(now.currentInstant))) throw new Error('participant local now');
  return { localDate: now.localDate, localTime: now.localTime, durationSeconds: '', occurrenceTouched: false };
}

export function overrideManualActivityOccurrence(
  state: ManualActivityFormState,
  occurrence: Partial<ManualActivityLocalDateTime>,
): ManualActivityFormState {
  return { ...state, ...occurrence, occurrenceTouched: true };
}

export function updateManualActivityDuration(
  state: ManualActivityFormState,
  durationSeconds: string,
  now: ManualActivityParticipantNow,
): ManualActivityFormState {
  const seconds = positiveWholeSeconds(durationSeconds);
  const nowValue = Date.parse(now.currentInstant);
  if (!Number.isFinite(nowValue)) throw new Error('participant current instant');
  if (state.occurrenceTouched || seconds === undefined) return { ...state, durationSeconds };
  const durationMilliseconds = seconds * 1_000;
  const occurrence = Number.isSafeInteger(durationMilliseconds)
    ? participantLocalDateTime(nowValue - durationMilliseconds, now.timeZone)
    : undefined;
  return occurrence
    ? { ...state, ...occurrence, durationSeconds }
    : { ...state, durationSeconds };
}

export function serializeManualActivityForm(
  state: ManualActivityFormState,
  now: ManualActivityParticipantNow,
): ManualActivityFormResult {
  const seconds = positiveWholeSeconds(state.durationSeconds);
  if (seconds === undefined) return { ok: false, reason: 'duration' };
  const occurrenceValue = participantInstantValue(state, now.timeZone);
  const nowValue = Date.parse(now.currentInstant);
  if (occurrenceValue === undefined || !Number.isFinite(nowValue)) return { ok: false, reason: 'occurrence' };
  if (seconds > (nowValue - occurrenceValue) / 1_000) {
    return { ok: false, reason: 'future_end' };
  }
  return {
    ok: true,
    fields: {
      localDate: state.localDate,
      localTime: state.localTime,
      durationSeconds: seconds,
    },
  };
}

export function createSessionOperationOwner(): SessionOperationOwner {
  let epoch = 0;
  return {
    issue: () => {
      const ownedEpoch = ++epoch;
      return { current: () => epoch === ownedEpoch };
    },
    invalidate: () => { epoch += 1; },
  };
}

export function declineDuplicateEmailRecovery(
  current: SessionCredential,
): SessionExchangeCredential & { nextAction: 'onboarding' } {
  if (current.nextAction !== 'duplicate_email_recovery') {
    throw new Error('duplicate_email_recovery session required');
  }
  return { token: current.token, expiresAt: current.expiresAt, nextAction: 'onboarding' };
}

export function classifySessionFailure(failure: SessionFailure): SessionFailureDecision {
  if (failure.kind === 'expired' || (failure.kind === 'http' && failure.status === 401)) {
    return { state: 'authentication_required', discardCredential: true, retryable: false };
  }
  if (failure.kind === 'local_storage') {
    return { state: 'local_session_unreadable', discardCredential: true, retryable: false };
  }
  if (failure.kind === 'network' || failure.status === 408 || failure.status === 429 || failure.status >= 500) {
    return { state: 'authenticated_offline', discardCredential: false, retryable: true };
  }
  return { state: 'authenticated_offline', discardCredential: false, retryable: false };
}

export function problemCode(value: unknown): ProblemCode | undefined {
  if (!value || typeof value !== 'object' || !('code' in value)) return undefined;
  const code = (value as { code?: unknown }).code;
  return typeof code === 'string' && (knownProblemCodes as readonly string[]).includes(code)
    ? code as ProblemCode
    : undefined;
}

export function sessionFailureFromResponse(status: number, problem?: unknown): SessionFailure {
  const code = problemCode(problem);
  return code ? { kind: 'http', status, code } : { kind: 'http', status };
}

export function isSessionFailure(value: unknown): value is SessionFailure {
  if (!value || typeof value !== 'object' || !("kind" in value)) return false;
  return ['network', 'http', 'expired', 'local_storage'].includes(String((value as { kind: unknown }).kind));
}

async function receiveSessionCredential<T extends SessionCredential>(
  request: () => Promise<SessionRefreshResponse>,
  decode: (data: unknown) => T | undefined,
  persist: (replacement: T) => Promise<void>,
): Promise<T> {
  let response: SessionRefreshResponse;
  try { response = await request(); }
  catch (cause) {
    if (isSessionFailure(cause)) throw cause;
    throw { kind: 'network' } satisfies SessionFailure;
  }
  if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
  let body: unknown;
  try { body = await response.json(); }
  catch { throw sessionFailureFromResponse(502); }
  const data = (body as { data?: unknown } | null)?.data;
  const replacement = decode(data);
  if (!replacement) {
    throw sessionFailureFromResponse(502);
  }
  try { await persist(replacement); }
  catch (cause) {
    if (isSessionFailure(cause)) throw cause;
    throw { kind: 'local_storage', reason: 'malformed' } satisfies SessionFailure;
  }
  return replacement;
}

export async function refreshSessionCredential(
  current: SessionCredential,
  request: (current: SessionCredential) => Promise<SessionRefreshResponse>,
  persist: (replacement: SessionCredential) => Promise<void>,
): Promise<SessionCredential> {
  return receiveSessionCredential(
    () => request(current),
    (data) => {
      if (!isValidSessionCredential(data)) return undefined;
      return current.nextAction
        ? { token: data.token, expiresAt: data.expiresAt, nextAction: current.nextAction }
        : { token: data.token, expiresAt: data.expiresAt };
    },
    persist,
  );
}

export async function exchangeSessionCredential(
  request: () => Promise<SessionRefreshResponse>,
  persist: (credential: SessionExchangeCredential) => Promise<void>,
): Promise<SessionExchangeCredential> {
  return receiveSessionCredential(
    request,
    (data) => isValidSessionCredential(data) && data.nextAction
      ? { token: data.token, expiresAt: data.expiresAt, nextAction: data.nextAction }
      : undefined,
    persist,
  );
}

export async function validateSessionCredential<T>(
  current: SessionCredential,
  request: (current: SessionCredential) => Promise<SessionRefreshResponse>,
): Promise<T> {
  let response: SessionRefreshResponse;
  try { response = await request(current); }
  catch (cause) {
    if (isSessionFailure(cause)) throw cause;
    throw { kind: 'network' } satisfies SessionFailure;
  }
  if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
  let body: unknown;
  try { body = await response.json(); }
  catch { throw sessionFailureFromResponse(502); }
  const data = (body as { data?: T } | null)?.data;
  if (!data) throw sessionFailureFromResponse(502);
  return data;
}

export async function validateSessionMutation(
  request: () => Promise<SessionRefreshResponse>,
): Promise<void> {
  let response: SessionRefreshResponse;
  try { response = await request(); }
  catch (cause) {
    if (isSessionFailure(cause)) throw cause;
    throw { kind: 'network' } satisfies SessionFailure;
  }
  if (!response.ok) throw sessionFailureFromResponse(response.status, response.problem);
}

export function sessionRetryDelay(attempt: number, expiresAt: string, now = Date.now()): number {
  const untilExpiry = Math.max(0, Date.parse(expiresAt) - now);
  const backoff = Math.min(300_000, 30_000 * (2 ** Math.max(0, Math.min(attempt, 10))));
  return Math.min(backoff, untilExpiry);
}

export function publicEndpointConfig(value: string | undefined, developmentDefault: string, name: string, production: boolean): string {
  const configured = value || (!production ? developmentDefault : '');
  if (!configured) throw new Error(name);
  if (!production) return configured;
  let parsed: URL;
  try { parsed = new URL(configured); } catch { throw new Error(name); }
  const hostname = parsed.hostname.toLowerCase().replace(/^\[|\]$/g, '');
  const privateIPv4Address = (octets: number[]) => octets.length === 4 && octets.every((part) => Number.isInteger(part) && part >= 0 && part <= 255) && (
    octets[0] === 0 || octets[0] === 10 || octets[0] === 127 ||
    (octets[0] === 169 && octets[1] === 254) ||
    (octets[0] === 172 && octets[1] >= 16 && octets[1] <= 31) ||
    (octets[0] === 192 && octets[1] === 168)
  );
  const privateIPv4 = privateIPv4Address(hostname.split('.').map(Number));
  const mapped = /^::ffff:([0-9a-f]{1,4}):([0-9a-f]{1,4})$/.exec(hostname);
  const mappedIPv4 = mapped ? privateIPv4Address([
    Number.parseInt(mapped[1], 16) >>> 8,
    Number.parseInt(mapped[1], 16) & 0xff,
    Number.parseInt(mapped[2], 16) >>> 8,
    Number.parseInt(mapped[2], 16) & 0xff,
  ]) : false;
  const privateIPv6 = hostname.includes(':') && (hostname === '::' || hostname === '::1' || hostname.startsWith('fc') || hostname.startsWith('fd') || hostname.startsWith('fe8') || hostname.startsWith('fe9') || hostname.startsWith('fea') || hostname.startsWith('feb'));
  const local = hostname === 'localhost' || hostname.endsWith('.localhost') || hostname.endsWith('.local') || privateIPv4 || mappedIPv4 || privateIPv6;
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash || local) throw new Error(name);
  return parsed.toString().replace(/\/$/, '');
}

export function publicEnvironmentConfig(value: string | undefined, name: string): 'development' | 'production' {
  const environment = value || 'development';
  if (environment !== 'development' && environment !== 'production') throw new Error(name);
  return environment;
}

export function publicStringConfig(value: string | undefined, developmentDefault: string, name: string, production: boolean): string {
  const configured = value?.trim() || (!production ? developmentDefault : '');
  if (!configured) throw new Error(name);
  return configured;
}

export function clientRuntimeConfig(value: unknown): ClientRuntimeConfig {
  if (!value || typeof value !== 'object') throw new Error('HOURPATHS_CLIENT_CONFIG');
  const input = value as Record<string, unknown>;
  if (typeof input.environment !== 'string' || input.environment.length === 0) throw new Error('HOURPATHS_APP_ENV');
  if (typeof input.apiURL !== 'string') throw new Error('HOURPATHS_API_URL');
  if (typeof input.oidcIssuer !== 'string') throw new Error('HOURPATHS_OIDC_ISSUER');
  if (typeof input.oidcClientId !== 'string') throw new Error('HOURPATHS_OIDC_CLIENT_ID');
  const environment = publicEnvironmentConfig(input.environment, 'HOURPATHS_APP_ENV');
  const production = environment === 'production';
  const apiURL = publicEndpointConfig(
    input.apiURL,
    'http://localhost:8080',
    'HOURPATHS_API_URL',
    production,
  );
  const oidcIssuer = publicEndpointConfig(
    input.oidcIssuer,
    'http://localhost:5556/dex',
    'HOURPATHS_OIDC_ISSUER',
    production,
  );
  const oidcClientId = publicStringConfig(
    input.oidcClientId,
    'hourpaths-client',
    'HOURPATHS_OIDC_CLIENT_ID',
    production,
  );
  if (/\s/.test(oidcClientId)) throw new Error('HOURPATHS_OIDC_CLIENT_ID');
  return { environment, apiURL, oidcIssuer, oidcClientId };
}
export function isValidSessionCredential(value: unknown): value is SessionCredential {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<SessionCredential>;
  return typeof candidate.token === 'string' && candidate.token.length > 0 &&
    typeof candidate.expiresAt === 'string' && Number.isFinite(Date.parse(candidate.expiresAt)) &&
    (!('nextAction' in candidate) || sessionNextActions.includes(candidate.nextAction as SessionNextAction));
}

export function retainedSessionExpiry(value: unknown, fallback: string): string {
  return isValidSessionCredential(value) ? value.expiresAt : fallback;
}

export {
  applyPathArchiveResult,
  createPathArchiveOperationOwner,
  pathArchiveLists,
  reviewPathArchiveChange,
  type PathArchiveMutationResult,
  type PathArchivePath,
  type PathArchiveRequestBody,
  type PathArchiveReview,
} from './path-archive';

export {
  applyPathDeletionResult,
  createPathDeletionOperationOwner,
  reviewPathDeletion,
  type PathDeletionMutationResult,
  type PathDeletionPath,
  type PathDeletionReceipt,
  type PathDeletionRequestBody,
  type PathDeletionReview,
} from './path-deletion';
export {
  createPathInvitationAcceptOwner,
  createPathInvitationRejectOwner,
  createPathInvitationRecipientReviewOwner,
  createPathInvitationSendOwner,
  mergePendingInvitationPage,
  pathInvitationFailureFromProblem,
  pathInvitationFailureMessageKey,
  pathInvitationOutputData,
  reviewPendingPathInvitationAcceptance,
  type PathInvitation,
  type PathInvitationAcceptanceReview,
  type PathInvitationAcceptBody,
  type PathInvitationAcceptResult,
  type PathInvitationAcceptSubmissionResult,
  type PathInvitationFailure,
  type PathInvitationFailureMessageKey,
  type PathInvitationProblemCode,
  type PathInvitationRecipient,
  type PathInvitationRecipientReview,
  type PathInvitationRejection,
  type PathInvitationRejectResult,
  type PathInvitationReviewResult,
  type PathInvitationRole,
  type PathInvitationSendBody,
  type PathInvitationSendResult,
  type PathInvitationVisibilityWarning,
  type PendingPathInvitation,
  type PendingPathInvitationPage,
  type PendingPathInvitationState,
} from './path-invitations';
export {
  createPathInvitationCancelOwner,
  mergeManagedPendingInvitationPage,
  type ManagedPendingPathInvitation,
  type ManagedPendingPathInvitationPage,
  type ManagedPendingPathInvitationState,
  type PathInvitationCancellation,
  type PathInvitationCancelResult,
} from './managed-path-invitations';
export {
  effectivePathCapabilities,
  pathsRequiringTimerRestore,
  type CapabilityPath,
  type PathCapabilities,
} from './path-capabilities';
export {
  applyPathRenameResult,
  createPathRenameOperationOwner,
  reviewPathRename,
  type PathRenameMutationResult,
  type PathRenamePath,
  type PathRenameRequestBody,
  type PathRenameReview,
} from './path-rename';
export {
  mergeNotificationHistoryPage,
  applyNotificationMutation,
  notificationPresentationMessageKey,
  type NotificationMutation,
  type NotificationMutationState,
  type NudgeNotification,
  type NudgeNotificationType,
  type NotificationPracticeReaction,
  type NotificationHistoryPage,
  type NotificationHistoryItem,
  type NotificationHistoryState,
  type NotificationPathRole,
  type NotificationPresentation,
  type NotificationPresentationMessageKey,
  type NotificationPublicIdentity,
  type PathInvitationNotification,
  type PathInvitationNotificationType,
  type PathOwnershipTransferNotificationType,
  type PathVisibilityChangedNotificationType,
  type SocialNotificationType,
} from './notification-history';
export {
  NUDGE_AUDIENCES,
  NUDGE_PRESETS,
  NUDGE_UNAVAILABLE_REASONS,
  createNudgeAudienceOperationOwner,
  createNudgeChannelOperationOwner,
  createNudgeSendOperationOwner,
  nudgeAudiencePreferenceFromAPI,
  nudgeChannelPreferenceFromAPI,
  nudgeEligibilityFromAPI,
  nudgeReceiptFromAPI,
  reviewNudgeAudienceChange,
  reviewNudgeChannelChange,
  reviewNudgeSend,
  type NudgeAudience,
  type NudgeAudiencePreference,
  type NudgeAudienceResult,
  type NudgeAudienceReview,
  type NudgeAudienceUpdateBody,
  type NudgeChannelPreference,
  type NudgeChannelResult,
  type NudgeChannelReview,
  type NudgeChannelUpdateBody,
  type NudgeContent,
  type NudgeEligibility,
  type NudgePreset,
  type NudgeReceipt,
  type NudgeSendBody,
  type NudgeSendResult,
  type NudgeSendReview,
  type NudgeUnavailableReason,
} from './nudges';
export {
  appendOptimisticPracticeComment,
  mergePracticeCommentPage,
  mergePracticeCommentHistoryPage,
  practiceCommentFromAPI,
  practiceCommentMutationFromAPI,
  practiceCommentPageFromAPI,
  practiceCommentHistoryPageFromAPI,
  removePracticeComment,
  restoreDeletedPracticeComment,
  rollbackPracticeCommentEdit,
  replacePracticeComment,
  updatePracticeCommentOptimistically,
  type PracticeComment,
  type PracticeCommentAuthor,
  type PracticeCommentPage,
  type PracticeCommentHistoryPage,
  type PracticeCommentVersion,
} from './practice-comments';
export {
  applyPracticeCommentHeartState,
  createPracticeCommentHeartOperationOwner,
  invalidatePracticeCommentHeartRoster,
  mergePracticeCommentHeartRosterPage,
  practiceCommentHeartRosterPageFromAPI,
  practiceCommentHeartStateFromAPI,
  rollbackPracticeCommentHeart,
  updatePracticeCommentHeartOptimistically,
  type PracticeCommentHearter,
  type PracticeCommentHeartMutationResult,
  type PracticeCommentHeartRosterPage,
  type PracticeCommentHeartState,
} from './practice-comment-hearts';
export {
  createProfileSearchOwner,
  mergeProfileSearchPage,
  profileSearchPageFromAPI,
  profileSearchQuery,
  publicProfileFromAPI,
  type ProfileSearchPage,
  type ProfileSearchResult,
  type ProfileSearchState,
  type ProfileRelationship,
  type PublicProfile,
} from './profile-discovery';
export {
  followRequestPageFromAPI,
  followRequestReviewResultFromAPI,
  mergeFollowRequestPage,
  relationshipMutationResultFromAPI,
  removeResolvedFollowRequest,
  type FollowRequest,
  type FollowRequestState,
  type FollowRequestReviewResult,
  type RelationshipMutationResult,
} from './follow-relationships';
import type { CapabilityPath } from './path-capabilities';
export {
  createAsyncMutationBarrier,
  type AsyncMutationBarrier,
  type AsyncMutationLease,
} from './async-mutation-barrier';
export {
  blockedAccountPageFromAPI,
  blockReviewFromAPI,
  blockResultFromAPI,
  mergeBlockedAccountPage,
  unblockResultFromAPI,
  type BlockableIdentity,
  type BlockedAccount,
  type BlockedAccountPage,
  type BlockReview,
  type BlockReviewAcknowledgement,
  type BlockResult,
  type SharedPathSummary,
  type UnblockResult,
  type UserBlockingPort,
} from './user-blocking';
export {
  applyPathMemberRemovalResult,
  createPathMemberRemovalOperationOwner,
  reviewPathMemberRemoval,
  type PathMemberRemovalMutationResult,
  type PathMemberRemovalReceipt,
  type PathMemberRemovalRequestBody,
  type PathMemberRemovalReview,
  type PathMemberRemovalRole,
} from './path-member-removal';
export {
  applyPathMemberRoleChangeResult,
  createPathMemberRoleChangeOperationOwner,
  reviewPathMemberRoleChange,
  type PathMemberAccessRole,
  type PathMemberRoleChangeBody,
  type PathMemberRoleChangeReceipt,
  type PathMemberRoleChangeResult,
  type PathMemberRoleChangeReview,
} from './path-member-role-change';
export {
  applyPathVisibilityResult,
  comparePathVisibility,
  createPathVisibilityOperationOwner,
  pathVisibilityOptions,
  pathVisibilityFromAPI,
  reviewPathVisibilityChange,
  type PathVisibility,
  type PathVisibilityChangeBody,
  type PathVisibilityChangeReview,
  type PathVisibilityMutationResult,
  type PathVisibilityProjection,
  type ProfileVisibility,
} from './path-visibility';
