import { createPathInvitationRecipientReviewOwner, createPathInvitationSendOwner, createPathInvitationCancelOwner, pathInvitationFailureMessageKey, pathInvitationFailureFromProblem } from '@hourpaths/client-core';
import type { SharingRepository } from '../ports/sharing-repository';
import type { SharingCommands } from '../ports/sharing-commands';
export function sharedInvitationCommands(repository: SharingRepository, key: () => string): SharingCommands {
  const review = createPathInvitationRecipientReviewOwner(), send = createPathInvitationSendOwner(key), cancel = createPathInvitationCancelOwner(key);
  return {
    review: (pathId, username) => review.review(pathId, username, repository.recipient),
    send: (value, role) => send.submit(value, role, true, repository.send),
    cancel: (pathId, id) => cancel.submit(pathId, id, true, repository.cancel),
    failureMessage: failure => pathInvitationFailureMessageKey(failure.kind === 'http' ? pathInvitationFailureFromProblem(failure.status, { code: failure.code }) : failure),
    clearReview: () => { review.cancel(); send.cancel(); },
    clearCancellation: () => cancel.cancel(),
    dispose: () => { review.cancel(); send.cancel(); cancel.cancel(); },
  };
}
