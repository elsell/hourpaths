export type IdentityProvider = 'google' | 'apple';
export interface LinkedProvider { provider: IdentityProvider; canUnlink: boolean }
export interface IdentityLinkChallenge { id: string; provider: IdentityProvider; nonce: string; expiresAt: number }
export interface IdentityLinkIntent extends IdentityLinkChallenge { owner: string }

export interface ProviderIdentityPorts {
  currentOwner(): string | null;
  now(): number;
  pending: {
    read(): Promise<IdentityLinkIntent | null>;
    save(intent: IdentityLinkIntent): Promise<void>;
    /** Compare both fields so an old callback cannot erase a newer intent. */
    remove(owner: string, id: string): Promise<void>;
  };
  remote: {
    list(): Promise<LinkedProvider[]>;
    begin(provider: IdentityProvider): Promise<IdentityLinkChallenge>;
    complete(id: string, token: string): Promise<void>;
    unlink(provider: IdentityProvider, reviewedOwner: string): Promise<void>;
  };
}

export function validIdentityLinkIntent(value: unknown): value is IdentityLinkIntent {
  if (!value || typeof value !== 'object') return false;
  const v = value as Partial<IdentityLinkIntent>;
  return typeof v.owner === 'string' && v.owner.length > 0 && v.owner.length <= 128
    && typeof v.id === 'string' && v.id.length > 0 && v.id.length <= 64
    && (v.provider === 'google' || v.provider === 'apple')
    && typeof v.nonce === 'string' && /^hourpaths-link:[a-f0-9]{64}$/.test(v.nonce)
    && typeof v.expiresAt === 'number' && Number.isFinite(v.expiresAt);
}

/** Shared account-bound orchestration. Provider authentication stays in native
 * and browser adapters; the resulting token is never stored by this service. */
export class ProviderIdentities {
  constructor(private readonly ports: ProviderIdentityPorts) {}

  private owner(): string {
    const owner = this.ports.currentOwner();
    if (!owner) throw new Error('sign_in_required');
    return owner;
  }
  private assertOwner(owner: string): void {
    if (this.ports.currentOwner() !== owner) throw new Error('account_changed');
  }
  async list(): Promise<LinkedProvider[]> {
    const owner = this.owner();
    const providers = await this.ports.remote.list();
    this.assertOwner(owner);
    return providers;
  }
  async begin(provider: IdentityProvider): Promise<IdentityLinkIntent> {
    const owner = this.owner();
    const challenge = await this.ports.remote.begin(provider);
    this.assertOwner(owner);
    const intent = { ...challenge, owner };
    if (!validIdentityLinkIntent(intent) || intent.provider !== provider || intent.expiresAt <= this.ports.now()) throw new Error('identity_link_invalid');
    await this.ports.pending.save(intent);
    if (this.ports.currentOwner() !== owner) {
      await this.ports.pending.remove(owner, intent.id);
      throw new Error('account_changed');
    }
    return intent;
  }
  async complete(token: string, expected?: { owner: string; id: string }): Promise<void> {
    const intent = await this.ports.pending.read();
    if (!validIdentityLinkIntent(intent)) throw new Error('identity_link_missing');
    this.assertOwner(intent.owner);
    if (expected && (intent.owner !== expected.owner || intent.id !== expected.id)) throw new Error('identity_link_superseded');
    if (intent.expiresAt <= this.ports.now()) throw new Error('identity_link_expired');
    if (!token) throw new Error('identity_token_missing');
    await this.ports.remote.complete(intent.id, token);
    await this.ports.pending.remove(intent.owner, intent.id);
    this.assertOwner(intent.owner);
  }
  async cancel(intent: IdentityLinkIntent): Promise<void> {
    await this.ports.pending.remove(intent.owner, intent.id);
  }
  async unlink(provider: IdentityProvider, reviewedOwner: string): Promise<void> {
    this.assertOwner(reviewedOwner);
    await this.ports.remote.unlink(provider, reviewedOwner);
    this.assertOwner(reviewedOwner);
  }
}

export interface ProviderSettingsService {
  owner(): string | null;
  list(): Promise<LinkedProvider[]>;
  link(provider: IdentityProvider): Promise<void>;
  unlink(provider: IdentityProvider, reviewedOwner: string): Promise<void>;
}

/** Dex and other development issuers need not support Logto's extended scope. */
export function providerSignInScopes(supported: readonly string[] | undefined): string[] {
  return supported?.includes('identities') ? ['openid', 'profile', 'email', 'identities'] : ['openid', 'profile', 'email'];
}
