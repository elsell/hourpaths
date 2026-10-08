import { createSessionApiClient } from '@hourpaths/api-client';
import { isValidSessionCredential } from '../index';
import type { AccountRecoveryPorts } from '../account-recovery';
import { ProviderIdentityFailure } from './api-provider-identities';

function failure(status: number): ProviderIdentityFailure {
  return new ProviderIdentityFailure(status === 401 ? 'authentication' : status === 400 ? 'proof' : status === 409 ? 'conflict' : status === 404 ? 'expired' : 'unavailable');
}
/** The token is frozen at construction; a delayed response must never make a
 * request with, or invalidate, a replacement account's credential. */
export function apiAccountRecovery(baseURL: string, token: string): ReturnType<AccountRecoveryPorts['remote']> {
  const api = createSessionApiClient(baseURL, () => token);
  return {
    async begin() {
      const result = await api.beginAccountRecovery();
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      const value = result.data.data;
      if (value.provider !== 'google' && value.provider !== 'apple') throw failure(502);
      return { id: value.id, nonce: value.nonce, provider: value.provider, expiresAt: Date.parse(value.expiresAt) };
    },
    async complete(id, proof) {
      const result = await api.completeAccountRecovery(id, proof);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      const value = result.data.data;
      if (!isValidSessionCredential(value) || value.nextAction !== 'home') throw failure(502);
      return { token: value.token, expiresAt: value.expiresAt, nextAction: 'home' };
    },
  };
}
