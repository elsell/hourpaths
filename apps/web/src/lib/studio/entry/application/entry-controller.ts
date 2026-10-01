import { activation, EntryFailure, type EntryContext, type EntryFields, type EntryState } from '../domain/entry';
import type { EntryService } from '../ports/entry-service';

/** Owns one account-entry lifetime; disposal invalidates every pending completion. */
export class EntryController {
  state: EntryState = { phase: 'loading', busy: false };
  private generation = 0;
  private disposed = false;
  private context: EntryContext | null = null;
  constructor(private readonly service: EntryService, private readonly now: () => number, private readonly changed: () => void) {}
  private publish(patch: Partial<EntryState>) { if (!this.disposed) { this.state = { ...this.state, ...patch }; this.changed(); } }
  private async run(work: (active: () => boolean) => Promise<void>) {
    if (this.disposed || this.state.busy) return;
    const generation = this.generation;
    const active = () => !this.disposed && generation === this.generation;
    this.publish({ busy: true, error: undefined });
    try { await work(active); }
    catch (error) {
      if (!active()) return;
      const kind = error instanceof EntryFailure ? error.kind : 'unavailable';
      if (kind === 'superseded') { this.context = null; this.publish({ phase: 'entry', review: undefined }); }
      else if (kind === 'expired' || kind === 'storage') { this.context = null; this.publish({ phase: 'entry', review: undefined, error: kind }); }
      else this.publish({ phase: this.state.phase === 'loading' ? 'entry' : this.state.phase, error: kind });
    } finally { if (active()) this.publish({ busy: false }); }
  }
  private async accept(context: EntryContext, active: () => boolean) {
    if (!active()) return;
    this.context = context; this.publish({ phase: context.kind });
    if (context.kind === 'home') { this.service.navigate('home'); return; }
    if (context.kind === 'onboarding') {
      const defaults = this.state.defaults ?? this.service.defaults();
      const review = await this.service.review();
      if (active()) this.publish({ review, defaults });
    }
  }
  initialize(callback = false) { return this.run(async active => {
    const context = await (callback ? this.service.callback() : this.service.restore());
    if (!active()) return;
    if (callback && context.kind !== 'entry') { this.context = context; this.service.navigate(context.kind); return; }
    await this.accept(context, active);
  }); }
  retry() { return this.initialize(); }
  begin() { return this.run(async () => { await this.service.begin(); }); }
  activate(fields: EntryFields, reviewToken: string) {
    if (this.state.phase !== 'onboarding') return Promise.resolve();
    this.maintain();
    if (this.state.phase !== 'onboarding') return Promise.resolve();
    return this.run(async active => {
      if (!this.state.review || reviewToken !== this.state.review.token) throw new EntryFailure('policy');
      const input = activation(fields, reviewToken);
      try { await this.accept(await this.service.activate(input), active); }
      catch (error) {
        if (!active()) return;
        if (error instanceof EntryFailure && error.kind === 'policy') {
          // Remove the old review before retrying a failed policy refresh.
          this.publish({ review: undefined, error: 'policy' });
          const review = await this.service.review();
          if (active()) this.publish({ review, error: 'policy' });
        } else throw error;
      }
    });
  }
  decline() {
    this.maintain();
    if (this.state.phase !== 'recovery') return Promise.resolve();
    return this.run(async active => { const next = await this.service.decline(); if (!active()) return; this.context = next; this.service.navigate(next.kind); });
  }
  async signOut() {
    if (this.disposed) return;
    this.generation++; this.publish({ busy: true });
    await this.service.signOut(); this.context = null;
    this.publish({ phase: 'entry', busy: false, review: undefined, error: undefined });
  }
  maintain() {
    if (this.disposed || !this.context || this.context.kind === 'entry') return;
    if (this.context.expiresAt! <= this.now() || !this.service.current()) {
      const expired = this.context.expiresAt! <= this.now();
      this.generation++; this.service.expire(); this.context = null;
      this.publish({ phase: 'entry', busy: false, review: undefined, error: expired ? 'expired' : undefined });
    }
  }
  openPolicy(url: string) { try { this.service.openPolicy(url); } catch { this.publish({ error: 'unavailable' }); } }
  clearError() { this.publish({ error: undefined }); }
  dispose() { this.disposed = true; this.generation++; this.service.dispose(); }
}
