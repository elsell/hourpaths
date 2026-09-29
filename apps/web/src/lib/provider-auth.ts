import { UserManager, WebStorageStateStore } from 'oidc-client-ts';

export type ApplicationDestination = '/' | '/onboarding' | '/account-recovery';

export function replaceApplicationLocation(destination: ApplicationDestination): void {
  switch (destination) {
    case '/': window.location.replace('/'); break;
    case '/onboarding': window.location.replace('/onboarding'); break;
    case '/account-recovery': window.location.replace('/account-recovery'); break;
  }
}

function manager(issuer: string, clientId: string): UserManager {
  return new UserManager({
    authority: issuer,
    client_id: clientId,
    redirect_uri: `${window.location.origin}/callback`,
    post_logout_redirect_uri: `${window.location.origin}/signed-out`,
    response_type: 'code',
    scope: 'openid profile email',
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage }),
    automaticSilentRenew: false,
    monitorSession: false,
  });
}

export async function beginProviderSignIn(issuer: string, clientId: string): Promise<void> {
  if (window.sessionStorage.getItem('hourpaths_provider_logout_pending')) {
    await beginProviderSignOut(issuer, clientId);
    return;
  }
  await manager(issuer, clientId).signinRedirect({ prompt: 'login' });
}

export async function completeProviderSignIn(issuer: string, clientId: string): Promise<string> {
  const provider = manager(issuer, clientId);
  try {
    const user = await provider.signinRedirectCallback();
    if (!user.id_token) throw new Error('identity_token_missing');
    return user.id_token;
  } finally {
    await provider.removeUser().catch(() => undefined);
    await provider.clearStaleState().catch(() => undefined);
  }
}

export async function beginProviderSignOut(issuer: string, clientId: string): Promise<void> {
  window.sessionStorage.setItem('hourpaths_provider_logout_pending', '1');
  const provider = manager(issuer, clientId);
  await provider.removeUser();
  await provider.signoutRedirect({ state: 'hourpaths_logout' });
}

export async function completeProviderSignOut(issuer: string, clientId: string): Promise<void> {
  const result = await manager(issuer, clientId).signoutRedirectCallback();
  if (result.userState !== 'hourpaths_logout') throw new Error('provider_logout_state_invalid');
  window.sessionStorage.removeItem('hourpaths_provider_logout_pending');
  replaceApplicationLocation('/');
}
