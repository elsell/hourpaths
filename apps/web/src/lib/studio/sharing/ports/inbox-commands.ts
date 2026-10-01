import type { PendingInvitation, InvitationDecision, InvitationDecisionResult } from '../domain/inbox';
import type { InvitationFailure } from '../domain/invitations';
export interface InboxCommands {
  respond(review: PendingInvitation, decision: InvitationDecision): Promise<InvitationDecisionResult>;
  failureMessage(failure: InvitationFailure): 'pathInvitation.unavailable' | 'pathInvitation.warningRequired' | 'pathInvitation.retry' | 'pathInvitation.rateLimited' | 'pathInvitation.dependencyUnavailable' | 'pathInvitation.failure';
  clear(): void;
  dispose(): void;
}
