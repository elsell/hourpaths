import { reviewPathRename } from '@hourpaths/client-core';

export type PathRenamePresentation =
  | { canSave: false; kind: 'pristine' | 'invalid' | 'saving' }
  | { canSave: true; kind: 'dirty' }
  | { canSave: false; kind: 'saved'; name: string }
  | { canSave: false; kind: 'error'; text: string };

export type PathManagementScreen = 'overview' | 'name' | 'goals' | 'review' | 'visibility' | 'visibility-review' | 'delete';

type PathRenamePresentationInput = {
  busy: boolean;
  currentName: string;
  draftName: string;
  errorText?: string;
  savedName?: string;
};

export function derivePathRenamePresentation({
  busy,
  currentName,
  draftName,
  errorText,
  savedName,
}: PathRenamePresentationInput): PathRenamePresentation {
  if (busy) return { canSave: false, kind: 'saving' };
  if (errorText) return { canSave: false, kind: 'error', text: errorText };

  let review: ReturnType<typeof reviewPathRename>;
  try {
    review = reviewPathRename({
      capabilities: {
        inviteMembers: false,
        manageGoals: false,
      manageLifecycle: false,
      renamePath: true,
      trackTime: false,
      transferOwnership: false,
      },
      id: 'path-management-presentation',
      name: currentName,
    }, draftName);
  } catch {
    return { canSave: false, kind: 'invalid' };
  }
  if (savedName && savedName === review.expectedName && review.name === review.expectedName) {
    return { canSave: false, kind: 'saved', name: savedName };
  }
  if (!review.changed) return { canSave: false, kind: 'pristine' };
  return { canSave: true, kind: 'dirty' };
}

export function pathManagementIsBusy({
  deleteBusy,
  goalBusy,
  renameBusy,
  visibilityBusy = false,
}: {
  deleteBusy: boolean;
  goalBusy: boolean;
  renameBusy: boolean;
  visibilityBusy?: boolean;
}): boolean {
  return deleteBusy || goalBusy || renameBusy || visibilityBusy;
}

export function reconcilePathManagementScreen({
  goalSaved,
  hasReview,
  screen,
}: {
  goalSaved: boolean;
  hasReview: boolean;
  screen: PathManagementScreen;
}): PathManagementScreen {
  if (hasReview) return 'review';
  if (goalSaved && screen === 'review') return 'goals';
  return screen;
}
