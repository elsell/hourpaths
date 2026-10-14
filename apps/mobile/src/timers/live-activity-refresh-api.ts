import { createSessionApiClient, generatedResponse } from '@hourpaths/api-client';
import type { LiveActivityRefreshPort } from '@hourpaths/client-core';

type Credential = Readonly<{ owner: string; token: string; expiresAt: number }>;
/** Capture credentials only for their owner; never send an old token using a
 * replacement account's session. Expired credentials are discarded. */
export function createLiveActivityRefreshPort(baseURL: string, current: () => Credential | null, now: () => number): LiveActivityRefreshPort {
  const credentials = new Map<string, Credential>();
  function client(owner: string, signal: AbortSignal) {
    const active = current();
    if (active && active.expiresAt > now()) credentials.set(active.owner, active);
    for (const [key, credential] of credentials) if (credential.expiresAt <= now()) credentials.delete(key);
    const credential = credentials.get(owner);
    if (!credential) throw new Error('live_activity_session_unavailable');
    return createSessionApiClient(baseURL, () => credential.token, signal);
  }
  async function bounded<T>(owner: string, work: (api: ReturnType<typeof createSessionApiClient>) => Promise<T>): Promise<T> {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    try { return await work(client(owner, controller.signal)); }
    finally { clearTimeout(timeout); }
  }
  return {
    async register(value) {
      const active = current();
      if (active?.owner !== value.owner || active.expiresAt <= now()) throw new Error('live_activity_owner_changed');
      const result = generatedResponse(await bounded(value.owner, api => api.upsertLiveActivity(value.activityId, {
        accountLease: value.accountLease, token: value.token,
        expiresAt: new Date(value.expiresAt).toISOString(), boundaries: value.boundaries.map(time => new Date(time).toISOString()),
      })));
      if (!result.ok) throw new Error('live_activity_registration_unavailable');
    },
    async revoke(value) {
      const result = generatedResponse(await bounded(value.owner, api => api.revokeLiveActivity(value.activityId, value.accountLease)));
      if (!result.ok && result.status !== 404 && result.status !== 401) throw new Error('live_activity_revocation_unavailable');
    },
  };
}
