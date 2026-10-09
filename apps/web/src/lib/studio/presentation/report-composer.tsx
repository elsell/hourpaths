import type { MessageKey } from '@hourpaths/i18n';
import { createReportSubmissionOwner, reportReasons, ReportFailure, type ReportReason, type ReportReceipt, type ReportTarget } from '@hourpaths/client-core';
import { useEffect, useState } from 'react';
import { useBlocker } from '@tanstack/react-router';
import type { StudioDependencies } from './app';
import { ConfirmationDialog } from './confirmation-dialog';
import { ProfileBlocking } from './profile-blocking';
import { useOwnedOperation } from './use-owned-operation';

export function ReportAction({ target, dependencies: d, menu = false }: { target: ReportTarget; dependencies: StudioDependencies; menu?: boolean }) {
  const [open, setOpen] = useState(false);
  const action = <button onClick={() => setOpen(true)}>{d.i18n.t('reporting.action')}</button>;
  return <>{menu ? <details><summary aria-label={d.i18n.t('reporting.action')}>⋯</summary><div className="studio-action-menu">{action}</div></details> : action}
    {open && <ReportComposer key={JSON.stringify([d.accountScope, target.kind, target.id])} target={target} dependencies={d} close={() => setOpen(false)} />}
  </>;
}

function ReportComposer({ target, dependencies: d, close }: { target: ReportTarget; dependencies: StudioDependencies; close(): void }) {
  const operation = useOwnedOperation(d);
  const [owner] = useState(() => createReportSubmissionOwner(d.operationId));
  const [reason, setReason] = useState<ReportReason | ''>(''), [explanation, setExplanation] = useState('');
  const [busy, setBusy] = useState(false), [error, setError] = useState<MessageKey | null>(null);
  const [receipt, setReceipt] = useState<ReportReceipt | null>(null);
  useEffect(() => () => owner.cancel(), [owner]);
  useBlocker({ shouldBlockFn: () => busy, enableBeforeUnload: busy });
  async function submit() {
    if (!reason || busy) return;
    setBusy(true); setError(null);
    const result = await owner.submit({ target, reason, explanation }, (draft, key) => operation.run(signal => d.reporting.submit(draft, key, signal)));
    if (result.kind === 'busy' || !operation.active()) return;
    setBusy(false);
    if (result.kind === 'submitted') setReceipt(result.receipt);
    if (result.kind === 'failed') setError(result.cause instanceof ReportFailure && result.cause.kind === 'invalid' ? 'reporting.invalid' : result.cause instanceof ReportFailure && result.cause.kind === 'not_found' ? 'reporting.unavailable' : 'reporting.failed');
  }
  if (receipt) return <ConfirmationDialog title={d.i18n.t('reporting.sent')} busy={busy} hideCancel cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t('common.done')} cancel={close} confirm={close}>
    <p role="status">{d.i18n.t('reporting.acknowledgment')}</p>
    {receipt.blockTarget && <><p>{d.i18n.t('reporting.blockOffer')}</p><ProfileBlocking onBusyChange={setBusy} dependencies={d} profile={{ id: receipt.blockTarget.userId, username: receipt.blockTarget.username, name: receipt.blockTarget.displayName, relationship: 'none' }} /></>}
  </ConfirmationDialog>;
  return <ConfirmationDialog title={d.i18n.t('reporting.title')} busy={busy} confirmDisabled={!reason || Array.from(explanation.trim()).length > 1000} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t('reporting.submit')} cancel={close} confirm={() => void submit()}>
    <div className="studio-entry-form"><label>{d.i18n.t('reporting.chooseReason')}<select value={reason} disabled={busy} onChange={event => { setReason(event.target.value as ReportReason | ''); setError(null); }}>
      <option value="">{d.i18n.t('reporting.chooseReason')}</option>{reportReasons.map(value => <option key={value} value={value}>{d.i18n.t(`reporting.reason.${value}`)}</option>)}
    </select></label>
    <label>{d.i18n.t('reporting.explanation')}<textarea rows={3} value={explanation} disabled={busy} onChange={event => setExplanation(event.target.value)} /></label></div>
    <p>{d.i18n.t('reporting.limit')}</p>
    {busy && <p role="status">{d.i18n.t('reporting.submitting')}</p>}{error && <p role="alert">{d.i18n.t(error)}</p>}
  </ConfirmationDialog>;
}
