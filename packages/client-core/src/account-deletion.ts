export interface DeletionIntent {
  owner: string;
  receiptSecret: string;
  phase: 'pending' | 'confirmed';
}

export interface AccountDeletionPorts {
  currentOwner(): string | null;
  newSecret(): string;
  journal: {
    read(owner: string): Promise<DeletionIntent | null>;
    /** Atomically keep the first secret; only advance its phase, never replace it. */
    save(intent: DeletionIntent): Promise<DeletionIntent>;
    forget(owner: string): Promise<void>;
  };
  /** Persist a write/replay fence before any server-side removal is attempted. */
  fence(owner: string): Promise<void>;
  remote: {
    remove(owner: string, receiptSecret: string): Promise<void>;
    receipt(owner: string, receiptSecret: string): Promise<boolean>;
  };
  /** Remove only this owner's durable data and native notification/timer surfaces. */
  purge(owner: string): Promise<void>;
  /** Compare the owner before clearing; a replacement account must survive. */
  clearSession(owner: string): Promise<void>;
}

export function validDeletionIntent(value: unknown, owner: string): value is DeletionIntent {
  if (!value || typeof value !== 'object') return false;
  const intent = value as Partial<DeletionIntent>;
  return !!owner && owner.trim() === owner && owner.length <= 128 && intent.owner === owner
    && typeof intent.receiptSecret === 'string' && /^[a-f0-9]{64}$/.test(intent.receiptSecret)
    && (intent.phase === 'pending' || intent.phase === 'confirmed');
}

export class AccountDeletion {
  private readonly running = new Map<string, { kind: 'completion' | 'preparation'; promise: Promise<void> }>();
  constructor(private readonly ports: AccountDeletionPorts) {}

  confirm(owner: string): Promise<void> {
    if (!owner || this.ports.currentOwner() !== owner) return Promise.reject(new Error('account_changed'));
    return this.single(owner, async () => {
      let intent = await this.ports.journal.read(owner);
      if (!intent) {
        const candidate: DeletionIntent = { owner, receiptSecret: this.ports.newSecret(), phase: 'pending' };
        if (!validDeletionIntent(candidate, owner)) throw new Error('deletion_intent_invalid');
        intent = await this.ports.journal.save(candidate);
      }
      await this.complete(owner, intent);
    });
  }

  resume(owner: string): Promise<void> {
    return this.single(owner, async () => {
      const intent = await this.ports.journal.read(owner);
      if (intent) await this.complete(owner, intent);
    });
  }

  /** Local erasure is authorized by the durable intent, not by receipt availability.
   * Keep unresolved intent so a matching account can still retry server removal. */
  async prepareRecovery(owners: readonly string[], currentOwner = this.ports.currentOwner()): Promise<string[]> {
    for (const owner of owners) {
      await this.single(owner, async () => {
        const intent = await this.ports.journal.read(owner);
        if (!intent) return;
        if (!validDeletionIntent(intent, owner)) throw new Error('deletion_intent_invalid');
        await this.ports.fence(owner);
        await this.ports.purge(owner);
      }, 'preparation');
    }
    return owners.filter(owner => !currentOwner || owner === currentOwner);
  }

  private single(owner: string, work: () => Promise<void>, kind: 'completion' | 'preparation' = 'completion'): Promise<void> {
    const pending = this.running.get(owner);
    if (pending) {
      if (pending.kind === kind) return pending.promise;
      return pending.promise.catch(() => undefined).then(() => this.single(owner, work, kind));
    }
    const result = work().finally(() => { this.running.delete(owner); });
    this.running.set(owner, { kind, promise: result });
    return result;
  }

  private async complete(owner: string, intent: DeletionIntent): Promise<void> {
    if (!validDeletionIntent(intent, owner)) throw new Error('deletion_intent_invalid');
    await this.ports.fence(owner);
    if (intent.phase !== 'confirmed') {
      let confirmed = await this.ports.remote.receipt(owner, intent.receiptSecret);
      if (!confirmed) {
        if (this.ports.currentOwner() !== owner) throw new Error('deletion_sign_in_required');
        try {
          await this.ports.remote.remove(owner, intent.receiptSecret);
          confirmed = true;
        } catch (failure) {
          if (!await this.ports.remote.receipt(owner, intent.receiptSecret)) throw failure;
          confirmed = true;
        }
      }
      if (confirmed) {
        const saved = await this.ports.journal.save({ ...intent, phase: 'confirmed' });
        if (!validDeletionIntent(saved, owner) || saved.receiptSecret !== intent.receiptSecret || saved.phase !== 'confirmed') {
          throw new Error('deletion_confirmation_not_persisted');
        }
      }
    }
    await this.ports.purge(owner);
    await this.ports.clearSession(owner);
    await this.ports.journal.forget(owner);
  }
}
