import type { AccountExportController } from './account-export';
import type { PolicyTimersController } from './policy-timers';
import { createPolicyRenewalOwner, PolicyRenewalFailure, validPolicyReview, type PolicyConfirmation, type PolicyReview, type PolicyRenewalRepository } from './policy-renewal';
export interface PolicyReviewState {
  required: boolean;
  loading: boolean;
  saving: boolean;
  review: PolicyReview | null;
  error: 'unavailable' | 'review_changed' | 'invalid' | null;
}
export class PolicyReviewController {
  state: PolicyReviewState = { required: false, loading: false, saving: false, review: null, error: null };
  private listeners = new Set<() => void>();
  private epoch = 0;
  private disposed = false;
  private owner?: ReturnType<typeof createPolicyRenewalOwner>;
  private userId?: string;
  private stopRequired?: () => void;
  private stopExport?: () => void;
  constructor(private readonly repository: PolicyRenewalRepository, private readonly keyFactory: () => string, private readonly openLink: (url: string) => void | Promise<void> = () => { throw new PolicyRenewalFailure(); }, readonly timers?: PolicyTimersController, readonly exportData?: AccountExportController) {
    this.stopExport = exportData?.subscribe(() => this.update({}));
    this.stopRequired = repository.observeRequired?.(() => {
      if (this.disposed) return;
      this.update({ required: true });
      if (!this.state.loading && !this.state.saving) void this.refresh();
    });
  }
  subscribe(listener: () => void) { this.listeners.add(listener); return () => { this.listeners.delete(listener); }; }
  private update(patch: Partial<PolicyReviewState>) { if (!this.disposed) { this.state = { ...this.state, ...patch }; for (const listener of this.listeners) listener(); } }
  async refresh(): Promise<void> {
    if (this.disposed || this.state.saving) return;
    const generation = ++this.epoch;
    this.update({ loading: true, error: null });
    try {
      const review = await this.repository.review();
      if (this.disposed || generation !== this.epoch) return;
      if (!validPolicyReview(review) || this.userId && review.userId !== this.userId) throw new PolicyRenewalFailure();
      if (!this.owner) { this.userId = review.userId; this.owner = createPolicyRenewalOwner(review.userId, this.keyFactory); }
      this.update({ required: review.required, review, loading: false });
    } catch {
      if (!this.disposed && generation === this.epoch) this.update({ loading: false, error: 'unavailable' });
    }
  }
  async accept(confirmation: Omit<PolicyConfirmation, 'review'>): Promise<void> {
    if (this.disposed || this.state.saving || this.state.loading || !this.state.review || !this.owner) return;
    const generation = this.epoch;
    const review = this.state.review;
    this.update({ saving: true, error: null });
    const result = await this.owner.submit({ review, ...confirmation }, (value, key) => this.repository.accept(value, key));
    if (this.disposed || generation !== this.epoch) return;
    this.update({ saving: false });
    if (result.kind === 'applied') {
      // A replay may acknowledge an older publication. Re-read before allowing
      // normal use so a newer requirement cannot be bypassed by an old receipt.
      await this.refresh();
    } else if (result.kind === 'failed') {
      if (result.cause instanceof PolicyRenewalFailure && result.cause.kind === 'review_changed') {
        this.update({ review: null }); await this.refresh(); this.update({ error: 'review_changed' });
      } else this.update({ error: result.cause instanceof PolicyRenewalFailure && result.cause.kind === 'invalid' ? 'invalid' : 'unavailable' });
    }
  }
  async openPolicy(kind: 'terms' | 'privacy' | 'guidelines') {
    const review = this.state.review, generation = this.epoch;
    if (this.disposed || !review) return;
    try { await this.openLink(review.policies[kind].url); }
    catch { if (!this.disposed && generation === this.epoch) this.update({ error: 'unavailable' }); }
  }
  dispose() {
    this.disposed = true; this.epoch++;
    this.owner?.cancel(); this.timers?.dispose(); this.stopExport?.(); this.exportData?.dispose(); this.stopRequired?.(); this.listeners.clear();
  }
}
