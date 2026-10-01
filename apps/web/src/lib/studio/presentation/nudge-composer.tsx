import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useBlocker } from '@tanstack/react-router';
import type { StudioDependencies } from './app';
import type { PathMember } from '../sharing/domain/members';
import { presets, type Preset } from '../nudges/domain/nudge';
import { ConfirmationDialog } from './confirmation-dialog';
import { useOwnedOperation } from './use-owned-operation';
export function MemberEncouragement({ dependencies: d, pathId, pathName, member, disabled }: { dependencies: StudioDependencies; pathId: string; pathName: string; member: PathMember; disabled: boolean }) {
  const identity = useQuery({ queryKey: [d.accountScope, 'identity'], queryFn: ({ signal }) => d.preferences.identity(signal) });
  const [open, setOpen] = useState(false), [sent, setSent] = useState(false);
  const permitted = !!identity.data && identity.data.id !== member.userId && member.role !== 'supporter' && !member.blockedByViewer;
  const query = useQuery({ queryKey: [d.accountScope, 'nudgeEligibility', pathId, member.userId], enabled: permitted, queryFn: ({ signal }) => d.nudges.eligibility(pathId, member.userId, signal), staleTime: 0, refetchOnWindowFocus: true });
  if (!permitted) return null;
  const eligibility = !query.isError ? query.data : undefined;
  return <div>{sent && <p role="status">{d.i18n.t('nudge.compose.sent')}</p>}
    {query.isError ? <div role="alert"><p>{d.i18n.t('studio.nudgeEligibilityFailed')}</p><button disabled={disabled} onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <button disabled={disabled || query.isFetching || !eligibility?.eligible} onClick={() => { setOpen(true); setSent(false); }}>{d.i18n.t('nudge.sendAction')}</button>}
    {eligibility && !eligibility.eligible && <p>{d.i18n.t(eligibility.reason === 'goal_complete' ? 'nudge.eligibility.goalComplete' : 'nudge.eligibility.rateLimited')}</p>}
    {open && <NudgeComposer dependencies={d} pathId={pathId} pathName={pathName} member={member} close={() => setOpen(false)} sent={() => { setOpen(false); setSent(true); void query.refetch(); }} />}
  </div>;
}
function NudgeComposer({ dependencies: d, pathId, pathName, member, close, sent }: { dependencies: StudioDependencies; pathId: string; pathName: string; member: PathMember; close(): void; sent(): void }) {
  const operation = useOwnedOperation(d), client = useQueryClient();
  const [commands] = useState(() => d.nudges.commands(d.operationId));
  const [preset, setPreset] = useState<Preset | null>(null), [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  const admitted = useRef(false);
  const query = useQuery({ queryKey: [d.accountScope, 'nudgeEligibility', pathId, member.userId], queryFn: ({ signal }) => d.nudges.eligibility(pathId, member.userId, signal), staleTime: 0 });
  useEffect(() => () => commands.dispose(), [commands]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function send() {
    if (!preset || admitted.current || (!failed && (!query.data?.eligible || query.isError || query.isFetching))) return;
    admitted.current = true; setBusy(true);
    try {
      const result = await operation.run(signal => commands.send(pathId, member.userId, preset, signal));
      if (!operation.active()) return;
      if (result.kind === 'applied') { sent(); void client.invalidateQueries({ queryKey: [d.accountScope, 'pathMembers', pathId] }); }
      else if (result.kind === 'failed') setFailed(true);
    } catch { if (operation.active()) setFailed(true); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  return <ConfirmationDialog title={d.i18n.t('nudge.compose.title', { displayName: member.displayName })} busy={busy} confirmDisabled={!preset || (!failed && (query.isFetching || query.isError || !query.data?.eligible))} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(failed ? 'common.retry' : 'nudge.compose.send')} cancel={close} confirm={() => void send()}>
    <p>{d.i18n.t('nudge.compose.pathContext', { pathName })}</p>
    <fieldset disabled={busy} className="studio-choice-group"><legend>{d.i18n.t('nudge.compose.choosePreset')}</legend>{presets.map(value => <label key={value}><input type="radio" name="encouragement" checked={preset === value} onChange={() => { setPreset(value); setFailed(false); }} />{d.i18n.t(`nudge.preset.${value}`)}</label>)}</fieldset>
    {busy && <p role="status">{d.i18n.t('nudge.compose.sending')}</p>}
    {failed && <p role="alert">{d.i18n.t('nudge.compose.sendError')}</p>}
    {query.isError && !failed && <p role="alert">{d.i18n.t('studio.loadFailed')} <button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></p>}
    {query.data && !query.data.eligible && !failed && <p>{d.i18n.t(query.data.reason === 'goal_complete' ? 'nudge.eligibility.goalComplete' : 'nudge.eligibility.rateLimited')}</p>}
  </ConfirmationDialog>;
}
