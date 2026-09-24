export type PathNudgePreferenceTarget = Readonly<{
  ownerID: string;
  pathID: string;
  requestSessionToken: string;
  sessionTokens: readonly string[];
}>;

export function createPathNudgePreferenceTarget(
  ownerID: string,
  sessionToken: string,
  pathID: string,
): PathNudgePreferenceTarget {
  return {
    ownerID,
    pathID,
    requestSessionToken: sessionToken,
    sessionTokens: [sessionToken],
  };
}

export function ownsPathNudgePreferenceTarget(
  target: PathNudgePreferenceTarget | null,
  ownerID: string | null | undefined,
  sessionToken: string | null | undefined,
  pathID: string,
): boolean {
  return target !== null &&
    target.ownerID === ownerID &&
    target.pathID === pathID &&
    target.sessionTokens.includes(sessionToken ?? '');
}

export function rotatePathNudgePreferenceTarget(
  target: PathNudgePreferenceTarget,
  ownerID: string,
  sessionToken: string,
): PathNudgePreferenceTarget {
  if (target.ownerID !== ownerID || target.sessionTokens.includes(sessionToken)) return target;
  return { ...target, sessionTokens: [...target.sessionTokens, sessionToken] };
}

export type PathNudgeFailureDisposition = 'discard-current' | 'ignore' | 'offline' | 'unavailable';

export function pathNudgeFailureDisposition(input: Readonly<{
  discardCredential: boolean;
  latestOwnerID: string | null | undefined;
  latestSessionToken: string | null | undefined;
  pathID: string;
  requestSessionToken: string;
  retryable: boolean;
  target: PathNudgePreferenceTarget | null;
}>): PathNudgeFailureDisposition {
  if (!ownsPathNudgePreferenceTarget(
    input.target,
    input.latestOwnerID,
    input.latestSessionToken,
    input.pathID,
  )) return 'ignore';
  if (input.discardCredential) {
    return input.latestSessionToken === input.requestSessionToken ? 'discard-current' : 'offline';
  }
  return input.retryable ? 'offline' : 'unavailable';
}
