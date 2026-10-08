import type { SessionExchangeCredential } from './index';
import type { IdentityLinkChallenge } from './provider-identities';

/** An authentication lifecycle, not an account ID. Reauthenticating even as the
 * same person invalidates the old intent. No application credential is stored. */
export interface AccountRecoveryIntent extends IdentityLinkChallenge { lifecycle: string }
export interface AccountRecoveryPorts {
  lifecycle(): string | null;
  now(): number;
  pending: {
    read(): Promise<AccountRecoveryIntent | null>;
    save(intent: AccountRecoveryIntent): Promise<void>;
    remove(lifecycle: string, id: string): Promise<void>;
  };
  /** Must synchronously capture the exact onboarding credential for this lifecycle. */
  remote(lifecycle: string): {
    begin(): Promise<IdentityLinkChallenge>;
    complete(id: string, proof: string): Promise<SessionExchangeCredential>;
  };
  revoke(token: string): Promise<void>;
}
export function validAccountRecoveryIntent(value: unknown): value is AccountRecoveryIntent {
  if (!value || typeof value !== 'object') return false;
  const v = value as Partial<AccountRecoveryIntent>;
  return typeof v.lifecycle === 'string' && v.lifecycle.length > 0 && v.lifecycle.length <= 128
    && typeof v.id === 'string' && v.id.length > 0 && v.id.length <= 64
    && (v.provider === 'google' || v.provider === 'apple')
    && typeof v.nonce === 'string' && /^hourpaths-recovery:[a-f0-9]{64}$/.test(v.nonce)
    && typeof v.expiresAt === 'number' && Number.isFinite(v.expiresAt);
}
export class AccountRecovery {
  constructor(private readonly ports: AccountRecoveryPorts) {}
  private assertCurrent(lifecycle: string) {
    if (this.ports.lifecycle() !== lifecycle) throw new Error('account_changed');
  }
  async begin(): Promise<AccountRecoveryIntent> {
    const lifecycle = this.ports.lifecycle();
    if (!lifecycle) throw new Error('recovery_missing');
    const remote = this.ports.remote(lifecycle);
    const challenge = await remote.begin();
    this.assertCurrent(lifecycle);
    const intent = { ...challenge, lifecycle };
    if (!validAccountRecoveryIntent(intent) || intent.expiresAt <= this.ports.now()) throw new Error('recovery_invalid');
    await this.ports.pending.save(intent);
    if (this.ports.lifecycle() !== lifecycle) {
      await this.cancel(intent);
      throw new Error('account_changed');
    }
    return intent;
  }
  async complete(proof: string, expected: { lifecycle: string; id: string }): Promise<SessionExchangeCredential> {
    const intent = await this.ports.pending.read();
    if (!validAccountRecoveryIntent(intent)) throw new Error('recovery_missing');
    if (intent.lifecycle !== expected.lifecycle || intent.id !== expected.id) throw new Error('recovery_superseded');
    this.assertCurrent(intent.lifecycle);
    if (intent.expiresAt <= this.ports.now()) throw new Error('recovery_expired');
    if (!proof) throw new Error('identity_token_missing');
    const credential = await this.ports.remote(intent.lifecycle).complete(intent.id, proof);
    // The server has now consumed the provisional identity. Never leave an
    // unadopted credential live when the local lifecycle or storage has changed.
    try {
      this.assertCurrent(intent.lifecycle);
      await this.cancel(intent);
      this.assertCurrent(intent.lifecycle);
      return credential;
    } catch (error) {
      await this.ports.revoke(credential.token).catch(() => undefined);
      throw error;
    }
  }
  async cancel(intent: AccountRecoveryIntent): Promise<void> {
    await this.ports.pending.remove(intent.lifecycle, intent.id);
  }
}
