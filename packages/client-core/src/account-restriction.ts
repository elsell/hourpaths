/** Account-owned application state, independent of transport and presentation. */
export interface AccountRestriction {
  noticeId: string;
  action: 'suspension' | 'ban';
  reason: string;
  issuedAt: string;
  until?: string;
}
export interface AccountRestrictionReview { userId: string; restriction: AccountRestriction | null }
export interface AccountRestrictionRepository {
  review(signal?: AbortSignal): Promise<AccountRestrictionReview>;
  observeRestricted?(notify: () => void): () => void;
}
export interface AccountRestrictionState {
  required: boolean;
  loading: boolean;
  review: AccountRestrictionReview | null;
  error: 'unavailable' | null;
}
export function validAccountRestrictionReview(review: AccountRestrictionReview): boolean {
  if (!review.userId || review.userId.trim() !== review.userId) return false;
  const value = review.restriction;
  if (value === null) return true;
  if (!value || !value.noticeId || value.noticeId.trim() !== value.noticeId || !value.reason?.trim() || !Number.isFinite(Date.parse(value.issuedAt))) return false;
  return value.action === 'ban' ? value.until === undefined : value.action === 'suspension' && value.until !== undefined && Number.isFinite(Date.parse(value.until)) && Date.parse(value.until) > Date.parse(value.issuedAt);
}
export class AccountRestrictionController {
  state: AccountRestrictionState = { required: false, loading: false, review: null, error: null };
  private listeners = new Set<() => void>();
  private epoch = 0;
  private disposed = false;
  private stopRestricted?: () => void;
  constructor(private readonly repository: AccountRestrictionRepository, knownReview?: AccountRestrictionReview) {
    if (knownReview && validAccountRestrictionReview(knownReview)) this.state = { ...this.state, required: knownReview.restriction !== null, review: knownReview };
    this.stopRestricted = repository.observeRestricted?.(() => {
      if (this.disposed) return;
      this.update({ required: true });
      // Invalidate a review begun before this denial. It cannot reopen access.
      void this.refresh();
    });
  }
  subscribe(listener: () => void) { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; }
  private update(patch: Partial<AccountRestrictionState>) {
    if (this.disposed) return;
    this.state = { ...this.state, ...patch };
    for (const listener of this.listeners) listener();
  }
  async refresh(): Promise<void> {
    if (this.disposed) return;
    const generation = ++this.epoch;
    this.update({ loading: true, error: null });
    try {
      const review = await this.repository.review();
      if (this.disposed || generation !== this.epoch) return;
      if (!validAccountRestrictionReview(review) || this.state.review && review.userId !== this.state.review.userId) throw new Error('invalid_account_restriction_review');
      this.update({ required: review.restriction !== null, review, loading: false });
    } catch {
      if (!this.disposed && generation === this.epoch) this.update({ loading: false, error: 'unavailable' });
    }
  }
  dispose() { this.disposed = true; this.epoch++; this.stopRestricted?.(); this.listeners.clear(); }
}
