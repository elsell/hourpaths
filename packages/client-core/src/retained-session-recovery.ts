import type { SessionExchangeCredential } from './index';

type Recovery<T> = { kind: 'retained'; value: T } | { kind: 'unavailable' } | { kind: 'superseded' };

/** Recover only the locally verified owner of an unusable credential. The
 * storage port must compare owner and token atomically before removing it. */
export async function recoverRetainedSession<T>(dependencies: {
  credential: SessionExchangeCredential;
  current(): boolean;
  restore(owner: string, current: () => boolean): Promise<{ owner: string; value: T } | null>;
  pause(owner: string, expectedToken: string, current: () => boolean): Promise<boolean>;
}): Promise<Recovery<T>> {
  const { credential, current } = dependencies;
  if (!current()) return { kind: 'superseded' };
  const owner = credential.ownerId;
  if (credential.nextAction !== 'home' || !owner || owner.trim() !== owner || owner.length > 128) return { kind: 'unavailable' };
  const retained = await dependencies.restore(owner, current);
  if (!current()) return { kind: 'superseded' };
  if (!retained || retained.owner !== owner) return { kind: 'unavailable' };
  if (!await dependencies.pause(owner, credential.token, current) || !current()) return { kind: 'superseded' };
  return { kind: 'retained', value: retained.value };
}
