import { createSessionApiClient, type TokenProvider } from '@hourpaths/api-client';
import type { ProviderIdentityPorts, IdentityProvider } from '../provider-identities';

export type ProviderIdentityFailureKind = 'authentication' | 'proof' | 'conflict' | 'expired' | 'unavailable';
export class ProviderIdentityFailure extends Error {
  constructor(readonly kind: ProviderIdentityFailureKind) { super(`identity_${kind}`); }
}
function failure(status: number): ProviderIdentityFailure {
  return new ProviderIdentityFailure(status === 401 ? 'authentication' : status === 400 ? 'proof' : status === 409 ? 'conflict' : status === 404 ? 'expired' : 'unavailable');
}
function provider(value: unknown): IdentityProvider {
  if (value !== 'google' && value !== 'apple') throw new ProviderIdentityFailure('unavailable');
  return value;
}

export function apiProviderIdentities(baseURL: string, token: TokenProvider, rejected?: (token: string | null) => void): ProviderIdentityPorts['remote'] {
  const api = createSessionApiClient(baseURL, token, undefined, rejected);
  return {
    async list() {
      const result = await api.linkedProviders();
      if (!result.data || !result.response.ok) throw failure(result.response.status);
      const rows = result.data.data;
      if (!Array.isArray(rows) || rows.length < 1 || rows.length > 2 || new Set(rows.map(row => row.provider)).size !== rows.length) throw new ProviderIdentityFailure('unavailable');
      return rows.map(row => ({ provider: provider(row.provider), canUnlink: rows.length > 1 && row.canUnlink === true }));
    },
    async begin(selected) {
      const result = await api.beginIdentityLink(selected);
      if (!result.data || !result.response.ok) throw failure(result.response.status);
      const value = result.data.data;
      return { id: value.id, provider: provider(value.provider), nonce: value.nonce, expiresAt: Date.parse(value.expiresAt) };
    },
    async complete(id, proof) {
      const result = await api.finishIdentityLink(id, proof);
      if (result.response.status !== 204) throw failure(result.response.status);
    },
    async unlink(selected, owner) {
      const result = await api.unlinkProvider(selected, owner);
      if (result.response.status !== 204) throw failure(result.response.status);
    },
  };
}
