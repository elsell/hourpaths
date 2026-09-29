import { useEffect, useRef } from 'react';
import { useBlocker } from '@tanstack/react-router';
import type { Translator } from '@hourpaths/i18n';
export function UnsavedChanges({ dirty, i18n }: { dirty: boolean; i18n: Translator }) {
  const blocker = useBlocker({ shouldBlockFn: () => dirty, enableBeforeUnload: dirty, withResolver: true });
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => { if (blocker.status === 'blocked') dialog.current?.showModal(); }, [blocker.status]);
  return blocker.status === 'blocked' ? <dialog ref={dialog} className="studio-settings-guard" aria-labelledby="settings-unsaved-title" onCancel={event => { event.preventDefault(); blocker.reset(); }}> <div><h2 id="settings-unsaved-title">{i18n.t('studio.settings.unsaved')}</h2><p>{i18n.t('studio.settings.unsavedDetail')}</p><button autoFocus onClick={() => blocker.reset()}>{i18n.t('studio.settings.keepEditing')}</button><button onClick={() => blocker.proceed()}>{i18n.t('studio.settings.discard')}</button></div></dialog> : null;
}
