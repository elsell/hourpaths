import * as AuthSession from 'expo-auth-session';
import { ProviderIdentities, apiProviderIdentities, AccountRecovery, apiAccountRecovery, type AccountRecoveryIntent, type IdentityLinkChallenge, type IdentityLinkIntent, type ProviderSettingsService } from '@hourpaths/client-core';

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
        const active = intent;
        const proof = await authenticateProviderProof(options, active, () => options.current()?.owner === active.owner);
        if (!proof) { await service.cancel(active); return; }
        await service.complete(proof, { owner: active.owner, id: active.id });
      } catch (error) {
        if (intent) await service.cancel(intent);
        throw error;
      } finally { busy = false; }
    },
  };
}

/** Both account operations use the same ephemeral browser and independent PKCE
 * request. Ordinary sign-in never consumes these purpose-bound proofs. */
async function authenticateProviderProof(options: { issuer: string; clientId: string }, intent: IdentityLinkChallenge, current: () => boolean): Promise<string | null> {
  const assertCurrent = () => { if (!current()) throw new Error('account_changed'); };
  const discovery = await AuthSession.fetchDiscoveryAsync(options.issuer);
  assertCurrent();
  const redirectUri = AuthSession.makeRedirectUri({ scheme: 'hourpaths', path: 'callback' });
  const request = new AuthSession.AuthRequest({
    clientId: options.clientId, redirectUri, scopes: ['openid', 'profile', 'email', 'identities'],
    usePKCE: true, prompt: AuthSession.Prompt.Login, extraParams: { nonce: intent.nonce },
  });
  const response = await request.promptAsync(discovery, { preferEphemeralSession: true });
  assertCurrent();
  if (response.type === 'cancel' || response.type === 'dismiss') return null;
  if (response.type !== 'success' || !response.params.code || !request.codeVerifier) throw new Error('identity_proof_failed');
  const result = await AuthSession.exchangeCodeAsync({ clientId: options.clientId, code: response.params.code, redirectUri, extraParams: { code_verifier: request.codeVerifier } }, discovery);
  assertCurrent();
  if (!result.idToken) throw new Error('identity_token_missing');
  return result.idToken;
}

export async function recoverNativeAccount(options: {
  apiURL: string; issuer: string; clientId: string; token: string;
  current(): boolean;
  revoke(token: string): Promise<void>;
}) {
  let pending: AccountRecoveryIntent | null = null;
  const lifecycle = 'native-enrollment'; // Private to this one in-memory attempt.
  const service = new AccountRecovery({
    lifecycle: () => options.current() ? lifecycle : null,
    now: () => Date.now(),
    pending: {
      read: async () => pending,
      save: async value => { pending = value; },
      remove: async (key, id) => { if (pending?.lifecycle === key && pending.id === id) pending = null; },
    },
    remote: () => {
      if (!options.current()) throw new Error('account_changed');
      return apiAccountRecovery(options.apiURL, options.token);
    },
    revoke: options.revoke,
  });
  const intent = await service.begin();
  try {
    const proof = await authenticateProviderProof(options, intent, options.current);
    return proof ? await service.complete(proof, intent) : null;
  } finally { await service.cancel(intent); }
}
