import { consumeAccountDeletionEntry, prepareAccountDeletionSignIn } from './account-deletion-entry';
import { providerSignInScopes, validIdentityLinkIntent, validAccountRecoveryIntent, type AccountRecoveryPorts, type AccountRecoveryIntent, type ProviderIdentityPorts, type IdentityLinkIntent } from '@hourpaths/client-core';
import { UserManager, WebStorageStateStore } from 'oidc-client-ts';

export type ApplicationDestination = '/' | '/studio' | '/onboarding' | '/account-recovery';

export function replaceApplicationLocation(destination: ApplicationDestination): void {
  switch (destination) {
    case '/': window.location.replace('/'); break;
    case '/studio':
      if (consumeAccountDeletionEntry(window.sessionStorage)) window.location.replace('/studio/delete-account');
      else window.location.replace('/studio');
      break;
    case '/onboarding': window.location.replace('/onboarding'); break;
    case '/account-recovery': window.location.replace('/account-recovery'); break;
  }
}

function manager(issuer: string, clientId: string): UserManager {
  return new UserManager({
    authority: issuer,
    client_id: clientId,
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: window.location.origin,
    response_type: 'code',
    scope: 'openid profile email',
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
    automaticSilentRenew: false,
    monitorSession: false,
  });
}

export async function beginProviderSignIn(issuer: string, clientId: string): Promise<void> {
  forgetProviderLinkIntent();
  window.sessionStorage.removeItem(providerRecoveryIntentKey);
  prepareAccountDeletionSignIn(window.sessionStorage, window.location.pathname);
  const provider = manager(issuer, clientId);
  const metadata = await provider.metadataService.getMetadata();
  await provider.signinRedirect({ prompt: 'login', scope: providerSignInScopes(metadata.scopes_supported).join(' ') });
}

export async function beginProviderLink(issuer: string, clientId: string, intent: IdentityLinkIntent): Promise<void> {
  await manager(issuer, clientId).signinRedirect({
    prompt: 'login', scope: 'openid profile email identities', nonce: intent.nonce,
    state: { purpose: 'identity-link', owner: intent.owner, challengeId: intent.id },
  });
}

export async function completeProviderAuthentication(issuer: string, clientId: string): Promise<{ identityToken: string; state: unknown }> {
  const provider = manager(issuer, clientId);
  try {
    const user = await provider.signinRedirectCallback();
    if (!user.id_token) throw new Error('identity_token_missing');
    return { identityToken: user.id_token, state: user.state };
  } finally {
    await provider.removeUser().catch(() => undefined);
    await provider.clearStaleState().catch(() => undefined);
  }
}

export async function completeProviderSignIn(issuer: string, clientId: string): Promise<string> {
  const result = await completeProviderAuthentication(issuer, clientId);
  if (result.state && typeof result.state === 'object' && 'purpose' in result.state && (result.state.purpose === 'identity-link' || result.state.purpose === 'account-recovery')) throw new Error('identity_link_callback_requires_account_context');
  return result.identityToken;
}


// Keep browser storage and navigation in the checksum-reviewed provider adapter.
const providerLinkIntentKey = 'hourpaths.identity-link';
export function forgetProviderLinkIntent(owner?: string): void {
  try {
    const raw = window.sessionStorage.getItem(providerLinkIntentKey);
    if (!raw) return;
    let value: unknown;
    try { value = JSON.parse(raw); } catch { value = null; }
    if (!owner || !validIdentityLinkIntent(value) || value.owner === owner) window.sessionStorage.removeItem(providerLinkIntentKey);
  } catch { /* Inaccessible storage cannot authorize a link callback. */ }
}
export function providerLinkIntentStorage(): ProviderIdentityPorts['pending'] {
  const key = providerLinkIntentKey;
  const read = (): IdentityLinkIntent | null => {
    const raw = window.sessionStorage.getItem(key);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!validIdentityLinkIntent(parsed)) throw new Error('identity_link_invalid');
    return parsed;
  };
  return {
    read: async () => read(),
    async save(intent) { window.sessionStorage.setItem(key, JSON.stringify(intent)); },
    async remove(owner, id) { const value = read(); if (value?.owner === owner && value.id === id) window.sessionStorage.removeItem(key); },
  };
}
export function replaceProviderSettingsLocation(result: 'success' | 'failed'): void {
  if (result === 'success') window.location.replace('/studio/settings/account?identityLink=success');
  else window.location.replace('/studio/settings/account?identityLink=failed');
}

const providerRecoveryIntentKey = 'hourpaths.account-recovery';
export async function beginProviderRecovery(issuer: string, clientId: string, intent: AccountRecoveryIntent): Promise<void> {
  await manager(issuer, clientId).signinRedirect({
    prompt: 'login', scope: 'openid profile email identities', nonce: intent.nonce,
    state: { purpose: 'account-recovery', lifecycle: intent.lifecycle, challengeId: intent.id },
  });
}
export function providerRecoveryIntentStorage(): AccountRecoveryPorts['pending'] {
  const read = (): AccountRecoveryIntent | null => {
    const raw = window.sessionStorage.getItem(providerRecoveryIntentKey);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!validAccountRecoveryIntent(parsed)) throw new Error('recovery_invalid');
    return parsed;
  };
  return {
    read: async () => read(),
    save: async intent => { window.sessionStorage.setItem(providerRecoveryIntentKey, JSON.stringify(intent)); },
    remove: async (lifecycle, id) => { const value = read(); if (value?.lifecycle === lifecycle && value.id === id) window.sessionStorage.removeItem(providerRecoveryIntentKey); },
  };
}

export function forgetProviderRecoveryIntent(lifecycle: string): void {
  try {
    const raw = window.sessionStorage.getItem(providerRecoveryIntentKey);
    if (!raw) return;
    let value: unknown;
    try { value = JSON.parse(raw); } catch { value = null; }
    if (!validAccountRecoveryIntent(value) || value.lifecycle === lifecycle) window.sessionStorage.removeItem(providerRecoveryIntentKey);
  } catch { /* A later callback still requires the durable lifecycle revision. */ }
}
