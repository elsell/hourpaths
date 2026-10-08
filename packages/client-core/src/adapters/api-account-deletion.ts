import { createSessionApiClient, type TokenProvider } from '@hourpaths/api-client';
import type { AccountDeletionPorts } from '../account-deletion';

export function apiAccountDeletion(baseURL: string, token: TokenProvider, rejected?: (token: string | null) => void): AccountDeletionPorts['remote'] {
  const api = createSessionApiClient(baseURL, token, undefined, rejected);
  return {
    async remove(owner, receiptSecret) {
      const result = await api.deleteAccount({ confirmed: true, reviewedUserId: owner, receiptSecret });
      if (result.response.status !== 204) throw new Error('account_deletion_failed');
    },
    async receipt(owner, receiptSecret) {
      const result = await api.confirmAccountDeletion({ userId: owner, receiptSecret });
      if (result.response.status === 204) return true;
      if (result.response.status === 401) return false;
      throw new Error('account_deletion_receipt_unavailable');
    },
  };
}
