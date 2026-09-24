import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { useCallback, useEffect, useState } from 'react';

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
  }, discovery);
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
    try { await prompt(); }
    catch {
      setFailed(true);
      setBusy(false);
    }
  }, [busy, discovery, prompt, request]);
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
    retry,
    acknowledge,
  };
}
