import type { Invitation, PathInvitationRecipient, FailedCommand } from './invitations';
export interface PendingInvitation {
  readonly invitation: Invitation;
  readonly pathName: string;
  readonly inviter: PathInvitationRecipient;
  readonly warning?: { readonly pathVisibility: 'followers' | 'public'; readonly hasRetainedActivity: boolean };
}
export interface InvitationInboxPage { readonly items: readonly PendingInvitation[]; readonly nextCursor: string }
export type InvitationDecision = 'accept' | 'decline';
export type InvitationDecisionResult = FailedCommand
  | { readonly kind: 'accepted'; readonly pathId: string; readonly role: 'participant' | 'supporter' }
  | { readonly kind: 'rejected' };
