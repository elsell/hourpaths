import { useEffect, useSyncExternalStore } from 'react';
import { commentDraftAfterFailedSubmit, commentDraftHasMeaningfulText } from './comment-presentation';

type EditDraft = Readonly<{ baseline: string; draft: string; visible: boolean }>;
const composerDrafts = new Map<string, string>();
const editDrafts = new Map<string, EditDraft>();
const listeners = new Set<() => void>();
let draftsDisposed = false;
let draftGeneration = 0;
const emit = () => { for (const listener of listeners) listener(); };
const editKey = (eventID: string, commentID: string) => `${eventID}\u0000${commentID}`;

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function setPracticeCommentComposerDraft(eventID: string, draft: string) {
  draftsDisposed = false;
  if (draft) composerDrafts.set(eventID, draft);
  else composerDrafts.delete(eventID);
  emit();
}

export function restorePracticeCommentComposerDraftIfEmpty(eventID: string, draft: string, admittedGeneration: number) {
  if (draftsDisposed || admittedGeneration !== draftGeneration) return false;
  const restored = commentDraftAfterFailedSubmit(composerDrafts.get(eventID) ?? '', draft);
  composerDrafts.set(eventID, restored);
  emit();
  return true;
}

export function usePracticeCommentComposerDraft(eventID: string) {
  return useSyncExternalStore(subscribe, () => composerDrafts.get(eventID) ?? '', () => '');
}

function ensureEditDraft(eventID: string, commentID: string, baseline: string) {
  const key = editKey(eventID, commentID);
  let current = editDrafts.get(key);
  if (!current) {
    current = { baseline, draft: baseline, visible: false };
    editDrafts.set(key, current);
  }
  return current;
}

export function usePracticeCommentEditDraft(eventID: string, commentID: string, baseline: string) {
  const key = editKey(eventID, commentID);
  ensureEditDraft(eventID, commentID, baseline);
  const current = useSyncExternalStore(subscribe, () => editDrafts.get(key)!, () => ({ baseline, draft: baseline, visible: false }));
  useEffect(() => {
    if (current.visible || current.baseline === baseline) return;
    editDrafts.set(key, { baseline, draft: baseline, visible: false });
    emit();
  }, [baseline, current.baseline, current.visible, key]);
  return current;
}

export function beginPracticeCommentEdit(eventID: string, commentID: string, baseline: string) {
  draftsDisposed = false;
  const key = editKey(eventID, commentID);
  const current = ensureEditDraft(eventID, commentID, baseline);
  editDrafts.set(key, { ...current, visible: true });
  emit();
}

export function setPracticeCommentEditDraft(eventID: string, commentID: string, draft: string) {
  draftsDisposed = false;
  const key = editKey(eventID, commentID);
  const current = editDrafts.get(key);
  if (!current) return;
  editDrafts.set(key, { ...current, draft });
  emit();
}

export function clearPracticeCommentEditDraft(eventID: string, commentID: string) {
  editDrafts.delete(editKey(eventID, commentID));
  emit();
}

export function clearPracticeCommentRouteDrafts(eventID: string) {
  composerDrafts.delete(eventID);
  const prefix = `${eventID}\u0000`;
  for (const key of editDrafts.keys()) if (key.startsWith(prefix)) editDrafts.delete(key);
  emit();
}

export function clearAllPracticeCommentDrafts() {
  draftsDisposed = true;
  draftGeneration += 1;
  composerDrafts.clear();
  editDrafts.clear();
  emit();
}

export function practiceCommentComposerDraft(eventID: string) {
  return composerDrafts.get(eventID) ?? '';
}

export function practiceCommentDraftGeneration() {
  return draftGeneration;
}

export function practiceCommentRouteRemovalDecision(eventID: string, busy: boolean) {
  if (draftsDisposed) return 'allow' as const;
  if (busy) return 'block-busy' as const;
  return commentDraftHasMeaningfulText(practiceCommentComposerDraft(eventID))
    ? 'confirm-discard' as const
    : 'allow' as const;
}
