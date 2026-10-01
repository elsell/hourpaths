import { createPathInvitationRecipientReviewOwner, createPathInvitationSendOwner, createPathInvitationCancelOwner } from '@hourpaths/client-core';
import type { PathInvitationRecipientReview, PathInvitationRole } from '../domain/invitations';
import type { SharingRepository } from '../ports/sharing-repository';
export { pathInvitationFailureMessageKey } from '@hourpaths/client-core';
export function sharingCommands(repository: SharingRepository, key: () => string) {
  const review = createPathInvitationRecipientReviewOwner(), send = createPathInvitationSendOwner(key), cancel = createPathInvitationCancelOwner(key);
  return {
    review: (pathId: string, username: string) => review.review(pathId, username, repository.recipient),
    send: (value: PathInvitationRecipientReview, role: PathInvitationRole) => send.submit(value, role, true, repository.send),
    cancel: (pathId: string, id: string) => cancel.submit(pathId, id, true, repository.cancel),
    clearReview: () => { review.cancel(); send.cancel(); },
    clearCancellation: () => cancel.cancel(),
    dispose: () => { review.cancel(); send.cancel(); cancel.cancel(); },
  };
}

export function retainSharingDraft(cause: unknown): boolean {
  if (!cause || typeof cause !== 'object') return false;
  const failure = cause as { kind?: string; status?: number };
  return failure.kind === 'network' || (failure.kind === 'http' && (failure.status === 429 || (typeof failure.status === 'number' && failure.status >= 500)));
}
