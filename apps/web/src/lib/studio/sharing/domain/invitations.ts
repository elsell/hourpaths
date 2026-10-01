export interface SharingContext { readonly id: string; readonly name: string }
export type PathInvitationRole = 'participant' | 'supporter';
export interface PathInvitationRecipient { readonly userId: string; readonly username: string; readonly displayName: string }
export interface PathInvitationRecipientReview { readonly pathId: string; readonly requestedUsername: string; readonly recipient: PathInvitationRecipient }
export interface PathInvitationSendBody { readonly username: string; readonly expectedRecipientUserId: string; readonly offeredRole: PathInvitationRole }
export interface Invitation { readonly id: string; readonly pathId: string; readonly inviterUserId: string; readonly recipientUserId: string; readonly offeredRole: PathInvitationRole; readonly createdAt: string }
export interface ManagedPendingPathInvitation { readonly invitation: Invitation; readonly inviter: PathInvitationRecipient; readonly recipient: PathInvitationRecipient }
export interface ManagedPendingPathInvitationPage { readonly items: readonly ManagedPendingPathInvitation[]; readonly nextCursor: string }
export type InvitationFailure = { readonly kind: 'network' | 'opaque' | 'warning_required' | 'invalid_response' | 'unexpected' } | { readonly kind: 'http'; readonly status: number; readonly code?: string };
export type FailedCommand = { readonly kind: 'failed'; readonly failure: InvitationFailure } | { readonly kind: 'superseded' } | { readonly kind: 'cancelled' };
