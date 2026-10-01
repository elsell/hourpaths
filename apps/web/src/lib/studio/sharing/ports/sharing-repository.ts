import type { InvitationInboxPage } from '../domain/inbox';
import type { InboxCommands } from './inbox-commands';
import type { MemberCommands } from './member-commands';
import type { PathMember } from '../domain/members';
import type { SharingCommands } from './sharing-commands';
import type { SharingContext, PathInvitationRecipient, PathInvitationSendBody, ManagedPendingPathInvitationPage } from '../domain/invitations';
export interface SharingRepository {
  inbox(cursor: string, signal?: AbortSignal): Promise<InvitationInboxPage>;
  inboxCommands(key: () => string): InboxCommands;
  members(pathId: string, cursor: string, signal?: AbortSignal): Promise<{ items: readonly PathMember[]; nextCursor: string }>;
  memberCommands(key: () => string): MemberCommands;
  commands(key: () => string): SharingCommands;
  context(pathId: string, signal?: AbortSignal): Promise<SharingContext>;
  recipient(pathId: string, username: string): Promise<PathInvitationRecipient>;
  send(pathId: string, body: PathInvitationSendBody, key: string): Promise<unknown>;
  pending(pathId: string, cursor: string, signal?: AbortSignal): Promise<ManagedPendingPathInvitationPage>;
  cancel(pathId: string, invitationId: string, key: string): Promise<unknown>;
}
