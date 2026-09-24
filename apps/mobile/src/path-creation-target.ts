import type { PathVisibility, ProfileVisibility } from '@hourpaths/client-core';

export type PathCreationTarget = Readonly<{
  ownerID: string;
  requestSessionToken: string;
  sessionTokens: readonly string[];
}>;

export function ownsPathCreationTarget(
  target: PathCreationTarget | null,
  sessionToken: string | null | undefined,
  ownerID: string | null | undefined,
): boolean {
  return target !== null &&
    (target.requestSessionToken === sessionToken || target.sessionTokens.includes(sessionToken ?? '')) &&
    target.ownerID === ownerID;
}

export function rotatePathCreationTarget(
  target: PathCreationTarget,
  sessionToken: string,
): PathCreationTarget {
  return target.sessionTokens.includes(sessionToken)
    ? target
    : { ...target, sessionTokens: [...target.sessionTokens, sessionToken] };
}

export function defaultPathCreationVisibility(
  profileVisibility: ProfileVisibility | null | undefined,
): PathVisibility {
  if (profileVisibility === 'public') return 'public';
  if (profileVisibility === 'private') return 'followers';
  return 'private';
}
