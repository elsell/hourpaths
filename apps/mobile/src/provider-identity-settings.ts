import * as AuthSession from 'expo-auth-session';
import { ProviderIdentities, apiProviderIdentities, type IdentityLinkIntent, type ProviderSettingsService } from '@hourpaths/client-core';

/** Linking uses an independent PKCE request and never exchanges/replaces the
 * application session. A process restart cancels the local attempt safely. */
export function nativeProviderSettings(options: {
  apiURL: string; issuer: string; clientId: string;
  current(): { owner: string; token: string } | null;
}): ProviderSettingsService {
  let pending: IdentityLinkIntent | null = null;
  let busy = false;
  const service = new ProviderIdentities({
    currentOwner: () => options.current()?.owner ?? null,
    now: () => Date.now(),
    pending: {
      async read() { return pending; },
      async save(intent) { pending = intent; },
      async remove(owner, id) { if (pending?.owner === owner && pending.id === id) pending = null; },
    },
    remote: apiProviderIdentities(options.apiURL, () => options.current()?.token ?? null),
  });
  return {
    owner: () => options.current()?.owner ?? null,
    list: () => service.list(),
    unlink: (provider, owner) => service.unlink(provider, owner),
    async link(provider) {
      if (busy) throw new Error('identity_link_busy');
      busy = true;
      let intent: IdentityLinkIntent | null = null;
      try {
        intent = await service.begin(provider);
        const discovery = await AuthSession.fetchDiscoveryAsync(options.issuer);
        if (options.current()?.owner !== intent.owner) throw new Error('account_changed');
        const redirectUri = AuthSession.makeRedirectUri({ scheme: 'hourpaths', path: 'callback' });
        const request = new AuthSession.AuthRequest({
          clientId: options.clientId, redirectUri, scopes: ['openid', 'profile', 'email', 'identities'],
          usePKCE: true, prompt: AuthSession.Prompt.Login, extraParams: { nonce: intent.nonce },
        });
        const response = await request.promptAsync(discovery, { preferEphemeralSession: true });
        if (response.type === 'cancel' || response.type === 'dismiss') { await service.cancel(intent); return; }
        if (response.type !== 'success' || !response.params.code || !request.codeVerifier) throw new Error('identity_link_failed');
        if (options.current()?.owner !== intent.owner) throw new Error('account_changed');
        const result = await AuthSession.exchangeCodeAsync({ clientId: options.clientId, code: response.params.code, redirectUri, extraParams: { code_verifier: request.codeVerifier } }, discovery);
        if (!result.idToken) throw new Error('identity_token_missing');
        await service.complete(result.idToken, { owner: intent.owner, id: intent.id });
      } catch (error) {
        if (intent) await service.cancel(intent);
        throw error;
      } finally { busy = false; }
    },
  };
}
