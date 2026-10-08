import { ProviderIdentities, type IdentityLinkIntent, type ProviderIdentityPorts, type ProviderSettingsService } from './provider-identities';

/** Native proof attempts live only within one authenticated account lifetime.
 * Authentication adapters own PKCE; this workflow owns cancellation/admission. */
export function ephemeralProviderSettings(options: {
  current(): { owner: string; token: string } | null;
  now(): number;
  remote(token: string): ProviderIdentityPorts['remote'];
  authenticate(intent: IdentityLinkIntent, current: () => boolean): Promise<string | null>;
}): ProviderSettingsService & { invalidate(): void } {
  let generation = 0;
  let admission: object | null = null;
  function attempt() {
    const owner = options.current()?.owner;
    if (!owner) throw new Error('sign_in_required');
    const lifetime = generation;
    const current = () => lifetime === generation && options.current()?.owner === owner;
    const assertCurrent = () => { if (!current()) throw new Error('account_changed'); };
    // Freeze a credential for each request. Rotation is allowed between requests;
    // a transport cannot adopt credentials from a replacement authentication.
    const remote = () => {
      assertCurrent();
      return options.remote(options.current()!.token);
    };
    let pending: IdentityLinkIntent | null = null;
    const service = new ProviderIdentities({
      currentOwner: () => current() ? owner : null,
      now: options.now,
      pending: {
        read: async () => pending,
        save: async intent => { assertCurrent(); pending = intent; },
        remove: async (account, id) => { if (pending?.owner === account && pending.id === id) pending = null; },
      },
      remote: {
        list: () => remote().list(),
        begin: provider => remote().begin(provider),
        complete: (id, proof) => remote().complete(id, proof),
        unlink: (provider, reviewedOwner) => remote().unlink(provider, reviewedOwner),
      },
    });
    return { service, current, assertCurrent };
  }
  return {
    owner: () => options.current()?.owner ?? null,
    invalidate() { generation += 1; admission = null; },
    list: async () => attempt().service.list(),
    unlink: async (provider, owner) => attempt().service.unlink(provider, owner),
    async link(provider) {
      if (admission) throw new Error('identity_link_busy');
      const token = {};
      const active = attempt();
      admission = token;
      let intent: IdentityLinkIntent | null = null;
      try {
        intent = await active.service.begin(provider);
        active.assertCurrent();
        const proof = await options.authenticate(intent, active.current);
        active.assertCurrent();
        if (proof) await active.service.complete(proof, intent);
      } finally {
        if (intent) await active.service.cancel(intent);
        if (admission === token) admission = null;
      }
    },
  };
}
