import { useLayoutEffect, useId, useRef, type ReactNode } from 'react';
export function ConfirmationDialog({ title, children, busy, confirmDisabled = false, hideCancel = false, cancelLabel, confirmLabel, cancel, confirm }: { title: string; children: ReactNode; busy: boolean; confirmDisabled?: boolean; hideCancel?: boolean; cancelLabel: string; confirmLabel: string; cancel(): void; confirm(): void }) {
  const dialog = useRef<HTMLDialogElement>(null), cancelButton = useRef<HTMLButtonElement>(null), heading = useId();
  useLayoutEffect(() => {
    const element = dialog.current;
    element?.showModal();
    cancelButton.current?.focus();
    return () => { element?.close(); };
  }, []);
  return <dialog ref={dialog} className="studio-settings-guard" aria-labelledby={heading} onCancel={event => { event.preventDefault(); if (!busy) cancel(); }}><div><h2 id={heading}>{title}</h2>{children}<div className="studio-form-actions">{!hideCancel && <button ref={cancelButton} autoFocus disabled={busy} onClick={cancel}>{cancelLabel}</button>}<button autoFocus={hideCancel} disabled={busy || confirmDisabled} onClick={confirm}>{confirmLabel}</button></div></div></dialog>;
}
