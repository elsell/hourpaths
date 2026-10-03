import { SessionUnavailable, type Session } from '../domain/session';
import type { SessionService, SessionStore } from '../ports/session-store';

/** Owns one signed-in lifetime. A replacement account requires a new controller. */
export class SessionController {
  private current: Session | null;
  private generation = 0;
  private nextRefreshAttempt = 0;
  private renewable = true;
  private pending: Promise<Session> | null = null;

  constructor(initial: Session, private readonly store: SessionStore,
    private readonly service: SessionService, private readonly now: () => number,
    private readonly lost: () => void) { this.current = initial; }

  token(): string | null {
    if (!this.current) return null;
    if (this.store.read()?.token !== this.current.token) {
      this.invalidate(false);
      return null;
    }
    if (this.current.expiresAt <= this.now()) {
      this.invalidate(true, true);
      return null;
    }
    return this.current.token;
  }

  owner(): string | null { return this.token() ? this.current?.ownerId ?? null : null; }

  /** Called only with the owner returned by a successful authenticated profile
   * read. An old response cannot relabel a replacement session. */
  async bindOwner(expectedToken: string, ownerId: string): Promise<boolean> {
    if (!ownerId || ownerId.trim() !== ownerId || ownerId.length > 128) throw new SessionUnavailable(false);
    if (this.token() !== expectedToken || !this.current) return false;
    if (this.current.ownerId && this.current.ownerId !== ownerId) {
      this.invalidate(true);
      return false;
    }
    if (this.current.ownerId === ownerId) return true;
    const replacement = { ...this.current, ownerId };
    await this.store.write(replacement, expectedToken);
    if (!this.current || this.current.token !== expectedToken) return false;
    this.current = replacement;
    return true;
  }

  async maintain(): Promise<void> {
    if (!this.token() || !this.current || this.pending || !this.renewable || this.now() < this.nextRefreshAttempt || this.current.expiresAt - this.now() > 60_000) return;
    const previousExpiry = this.current.expiresAt;
    try {
      const replacement = await this.refresh();
      this.renewable = replacement.expiresAt > previousExpiry;
    } catch { this.nextRefreshAttempt = this.now() + 30_000; }
  }

  refresh(): Promise<Session> {
    if (this.pending) return this.pending;
    if (!this.token() || !this.current) return Promise.reject(new SessionUnavailable(false));
    const previous = this.current;
    const generation = this.generation;
    const refresh = async () => {
      if (generation !== this.generation || this.store.read()?.token !== previous.token) throw new SessionUnavailable(false);
      let next = await this.service.refresh(previous);
      if (generation !== this.generation || this.store.read()?.token !== previous.token) {
        await this.revoke(next);
        throw new SessionUnavailable(false);
      }
      if (next.expiresAt <= this.now()) {
        await this.revoke(next);
        throw new SessionUnavailable(false);
      }
      if (this.current?.ownerId) next = { ...next, ownerId: this.current.ownerId };
      try { await this.store.write(next, previous.token); }
      catch (error) {
        if (this.store.read()?.token !== next.token) await this.revoke(next);
        throw error;
      }
      if (generation !== this.generation) throw new SessionUnavailable(false);
      this.current = next;
      return next;
    };
    const operation = (this.store.refreshExclusive ? this.store.refreshExclusive(refresh) : refresh()).catch(error => {
      if (generation === this.generation &&
        (!(error instanceof SessionUnavailable) || !error.retryable || previous.expiresAt <= this.now())) {
        this.invalidate(this.store.read()?.token === previous.token, previous.expiresAt <= this.now() || error instanceof SessionUnavailable && error.reason === 'rejected');
      }
      throw error;
    }).finally(() => { if (this.pending === operation) this.pending = null; });
    this.pending = operation;
    return operation;
  }

  reject(credential: string | null): void {
    if (!credential || credential !== this.current?.token) return;
    this.invalidate(!!this.current && this.store.read()?.token === this.current.token, true);
  }

  dispose(): void {
    this.current = null;
    this.generation++;
  }

  signOut(): void {
    const previous = this.current;
    this.invalidate(!!previous && this.store.read()?.token === previous.token);
    if (previous) void this.revoke(previous);
  }

  private invalidate(clear: boolean, retain = false): void {
    if (!this.current) return;
    const owner = this.current.ownerId;
    const expectedToken = this.current.token;
    this.current = null;
    this.generation++;
    if (!clear) { this.lost(); return; }
    const removal = retain && owner && this.store.pause ? this.store.pause(owner, expectedToken) : this.store.clear(expectedToken);
    if (removal) void removal.then(() => this.lost(), () => this.lost());
    else this.lost();
  }

  private async revoke(session: Session): Promise<void> {
    try { await this.service.revoke(session); } catch { /* Local invalidation is authoritative. */ }
  }
}
