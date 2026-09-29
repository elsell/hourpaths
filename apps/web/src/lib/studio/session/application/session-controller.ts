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
      this.invalidate(true);
      return null;
    }
    return this.current.token;
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
    const operation = this.service.refresh(previous).then(async next => {
      if (generation !== this.generation || this.store.read()?.token !== previous.token) {
        await this.revoke(next);
        throw new SessionUnavailable(false);
      }
      if (next.expiresAt <= this.now()) {
        await this.revoke(next);
        throw new SessionUnavailable(false);
      }
      this.store.write(next);
      this.current = next;
      return next;
    }).catch(error => {
      if (generation === this.generation &&
        (!(error instanceof SessionUnavailable) || !error.retryable || previous.expiresAt <= this.now())) {
        this.invalidate(this.store.read()?.token === previous.token);
      }
      throw error;
    }).finally(() => { if (this.pending === operation) this.pending = null; });
    this.pending = operation;
    return operation;
  }

  reject(): void {
    this.invalidate(!!this.current && this.store.read()?.token === this.current.token);
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

  private invalidate(clear: boolean): void {
    if (!this.current) return;
    this.current = null;
    this.generation++;
    if (clear) this.store.clear();
    this.lost();
  }

  private async revoke(session: Session): Promise<void> {
    try { await this.service.revoke(session); } catch { /* Local invalidation is authoritative. */ }
  }
}
