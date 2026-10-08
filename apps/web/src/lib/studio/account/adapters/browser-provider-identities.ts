import { ProviderIdentities, apiProviderIdentities, type ClientRuntimeConfig, type ProviderSettingsService } from '@hourpaths/client-core';
import { createSessionApiClient } from '@hourpaths/api-client';
import { beginProviderLink, providerLinkIntentStorage } from '../../../provider-auth';
import { browserSessionStore } from '../../session/adapters/browser-session';

export function browserProviderIdentities(config: ClientRuntimeConfig): ProviderSettingsService & { complete(token: string, state: unknown): Promise<void> } {
  const sessions = browserSessionStore();
  const current = () => {
    const session = sessions.read();
    return session && session.destination === 'home' && session.expiresAt > Date.now() ? session : null;
  };
  const service = new ProviderIdentities({
    currentOwner: () => current()?.ownerId ?? null,
    now: () => Date.now(),
    pending: providerLinkIntentStorage(),
    remote: apiProviderIdentities(config.apiURL, () => current()?.token ?? null),
  });
  async function ensureOwner() {
    const session = current();
    if (!session) throw new Error('sign_in_required');
    if (session.ownerId) return;
    const result = await createSessionApiClient(config.apiURL, () => session.token).profile();
    if (!result.data?.data.id || !result.response.ok || current()?.token !== session.token) throw new Error('account_changed');
    await sessions.write({ ...session, ownerId: result.data.data.id }, session.token);
  }
  return {
    owner: () => current()?.ownerId ?? null,
    async list() { await ensureOwner(); return service.list(); },
    async link(provider) {
      await ensureOwner();
      const intent = await service.begin(provider);
      try { await beginProviderLink(config.oidcIssuer, config.oidcClientId, intent); }
      catch (error) { await service.cancel(intent); throw error; }
    },
    unlink: (provider, owner) => service.unlink(provider, owner),
    async complete(token, state) {
      if (!state || typeof state !== 'object' || !('purpose' in state) || state.purpose !== 'identity-link' || !('owner' in state) || typeof state.owner !== 'string' || !('challengeId' in state) || typeof state.challengeId !== 'string') throw new Error('identity_link_invalid');
      await service.complete(token, { owner: state.owner, id: state.challengeId });
    },
  };
}
