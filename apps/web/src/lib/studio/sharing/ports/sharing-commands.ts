import type { PathInvitationRecipientReview, PathInvitationRole, FailedCommand, InvitationFailure } from '../domain/invitations';
export interface SharingCommands {
  review(pathId: string, username: string): Promise<{ readonly kind: 'reviewed'; readonly review: PathInvitationRecipientReview } | FailedCommand>;
  send(review: PathInvitationRecipientReview, role: PathInvitationRole): Promise<{ readonly kind: 'sent' } | FailedCommand>;
  cancel(pathId: string, invitationId: string): Promise<{ readonly kind: 'canceled' } | FailedCommand>;
  failureMessage(failure: InvitationFailure): 'pathInvitation.unavailable' | 'pathInvitation.warningRequired' | 'pathInvitation.retry' | 'pathInvitation.rateLimited' | 'pathInvitation.dependencyUnavailable' | 'pathInvitation.failure';
  clearReview(): void;
  clearCancellation(): void;
  dispose(): void;
}
