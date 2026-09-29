import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import * as SecureStore from 'expo-secure-store';
import * as Crypto from 'expo-crypto';
import { createProviderLogout, validProviderLogoutReturn } from '@hourpaths/client-core';
import { useCallback, useEffect, useMemo, useState } from 'react';

import { loadProviderDiscovery } from './provider-discovery';
import { classifyProviderResponse, providerBusyAfterResponse } from './provider-auth-state';

WebBrowser.maybeCompleteAuthSession();

export type ProviderSignIn = {
  ready: boolean;
  busy: boolean;
  discoveryFailed: boolean;
  identityToken: string | null;
  failed: boolean;
  begin: () => Promise<void>;
  signOut: () => Promise<void>;
  retry: () => void;
  acknowledge: () => void;
};

export function useProviderSignIn(issuer: string, clientId: string, scheme: string): ProviderSignIn {
  const [discovery, setDiscovery] = useState<AuthSession.DiscoveryDocument | null>(null);
  const [discoveryAttempt, setDiscoveryAttempt] = useState(0);
  const [discoveryFailed, setDiscoveryFailed] = useState(false);
  const redirectUri = AuthSession.makeRedirectUri({ scheme, path: 'callback' });
  const [request, response, prompt] = AuthSession.useAuthRequest({
    clientId,
    redirectUri,
    scopes: ['openid', 'profile', 'email'],
    usePKCE: true,
    prompt: AuthSession.Prompt.Login,
  }, discovery);
  const logout = useMemo(() => createProviderLogout({
    pending: async () => await SecureStore.getItemAsync('hourpaths_provider_logout_pending') !== null,
    markPending: () => SecureStore.setItemAsync('hourpaths_provider_logout_pending', '1'),
    clearPending: () => SecureStore.deleteItemAsync('hourpaths_provider_logout_pending'),
    endSession: async () => {
      const document = discovery ?? await AuthSession.fetchDiscoveryAsync(issuer);
      if (!document.endSessionEndpoint) return false;
      const returnUri = AuthSession.makeRedirectUri({ scheme, path: 'logout' });
      const state = Crypto.randomUUID();
      const url = new URL(document.endSessionEndpoint);
      url.searchParams.set('client_id', clientId);
      url.searchParams.set('post_logout_redirect_uri', returnUri);
      url.searchParams.set('state', state);
      const result = await WebBrowser.openAuthSessionAsync(url.toString(), returnUri, { preferEphemeralSession: false });
      return result.type === 'success' && validProviderLogoutReturn(result.url, returnUri, state);
    },
  }), [clientId, discovery, issuer, scheme]);
  const [identityToken, setIdentityToken] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let active = true;
    setDiscovery(null);
    setDiscoveryFailed(false);
    void loadProviderDiscovery(() => AuthSession.fetchDiscoveryAsync(issuer)).then((next) => {
      if (active) {
        setDiscovery(next);
        setDiscoveryFailed(next === null);
      }
    });
    return () => { active = false; };
  }, [discoveryAttempt, issuer]);

  useEffect(() => {
    const responseState = classifyProviderResponse(response?.type);
    if (responseState === 'failed') {
      setIdentityToken(null);
      setFailed(true);
      setBusy(providerBusyAfterResponse(responseState));
      return;
    }
    if (responseState === 'cancelled') {
      setIdentityToken(null);
      setFailed(false);
      setBusy(false);
      return;
    }
    if (responseState !== 'success' || response?.type !== 'success' || !request?.codeVerifier || !discovery) return;
    void AuthSession.exchangeCodeAsync({
      clientId,
      code: response.params.code,
      redirectUri,
      extraParams: { code_verifier: request.codeVerifier },
    }, discovery).then((result) => {
      if (!result.idToken) throw new Error('identity_token_missing');
      setIdentityToken(result.idToken);
    }).catch(() => {
      setFailed(true);
      setBusy(false);
    });
  }, [clientId, discovery, redirectUri, request?.codeVerifier, response]);

  const begin = useCallback(async () => {
    if (busy || !request || !discovery) return;
    setFailed(false);
    setIdentityToken(null);
    setBusy(true);
    try {
      if (!await logout.beforeSignIn()) throw new Error('provider_logout_incomplete');
      await prompt({ preferEphemeralSession: true });
    }
    catch {
      setFailed(true);
      setBusy(false);
    }
  }, [busy, discovery, logout, prompt, request]);
  const signOut = useCallback(async () => {
    setIdentityToken(null);
    setBusy(true);
    setFailed(false);
    try { if (!await logout.signOut()) setFailed(true); }
    finally { setBusy(false); }
  }, [logout]);
  const retry = useCallback(() => {
    if (busy) return;
    setDiscoveryAttempt((attempt) => attempt + 1);
  }, [busy]);
  const acknowledge = useCallback(() => {
    setFailed(false);
    setIdentityToken(null);
    setBusy(false);
  }, []);
  return {
    ready: Boolean(request && discovery),
    busy,
    discoveryFailed,
    identityToken,
    failed,
    begin,
    signOut,
    retry,
    acknowledge,
  };
}
