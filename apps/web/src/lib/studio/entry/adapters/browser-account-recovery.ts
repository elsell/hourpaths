import { AccountRecovery, apiAccountRecovery, type ClientRuntimeConfig, type SessionOperationTicket } from '@hourpaths/client-core';
import { createSessionApiClient } from '@hourpaths/api-client';
import { applicationSession, persistOwnedApplicationSession, revokeSupersededApplicationSession } from '../../../auth';
import { browserSessionState } from '../../../browser-session-state';
import { beginProviderRecovery, providerRecoveryIntentStorage } from '../../../provider-auth';

export function recoveryCallback(value: unknown): { lifecycle: string; id: string } {
  if (!value || typeof value !== 'object' || !('purpose' in value) || value.purpose !== 'account-recovery'
    || !('lifecycle' in value) || typeof value.lifecycle !== 'string' || !value.lifecycle || value.lifecycle.length > 128
    || !('challengeId' in value) || typeof value.challengeId !== 'string' || !value.challengeId || value.challengeId.length > 64) throw new Error('recovery_invalid');
  return { lifecycle: value.lifecycle, id: value.challengeId };
}
export function browserAccountRecovery(config: ClientRuntimeConfig) {
  const current = () => {
    const value = applicationSession();
    return value?.nextAction === 'duplicate_email_recovery' && Date.parse(value.expiresAt) > Date.now() ? value : null;
  };
  const service = new AccountRecovery({
    lifecycle: () => current() ? browserSessionState().revision() : null,
    now: () => Date.now(),
    pending: providerRecoveryIntentStorage(),
    remote: lifecycle => {
      const session = current();
      if (!session || browserSessionState().revision() !== lifecycle) throw new Error('account_changed');
      return apiAccountRecovery(config.apiURL, session.token);
    },
    revoke: async token => { await createSessionApiClient(config.apiURL, () => token).revoke(); },
  });
  return {
    async begin() {
      const intent = await service.begin();
      try { await beginProviderRecovery(config.oidcIssuer, config.oidcClientId, intent); }
      catch (error) { await service.cancel(intent); throw error; }
    },
    async complete(token: string, state: unknown, ticket: SessionOperationTicket) {
      if (!ticket.current()) throw new Error('account_changed');
      const credential = await service.complete(token, recoveryCallback(state));
      try {
        if (!await persistOwnedApplicationSession(credential, ticket) || !ticket.current()) throw new Error('account_changed');
      } catch (error) { revokeSupersededApplicationSession(config, credential); throw error; }
    },
  };
}
