import type { SharingContext, PathInvitationRecipient, PathInvitationSendBody, ManagedPendingPathInvitationPage } from '../domain/invitations';
export interface SharingRepository {
  context(pathId: string, signal?: AbortSignal): Promise<SharingContext>;
  recipient(pathId: string, username: string): Promise<PathInvitationRecipient>;
  send(pathId: string, body: PathInvitationSendBody, key: string): Promise<unknown>;
  pending(pathId: string, cursor: string, signal?: AbortSignal): Promise<ManagedPendingPathInvitationPage>;
  cancel(pathId: string, invitationId: string, key: string): Promise<unknown>;
}
