import { useEffect, useRef, useState } from 'react';
import { Link, Navigate, useBlocker } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import { blockingCommands } from '../blocking/application/commands';
import type { BlockReview } from '../blocking/domain/block';
import type { Profile } from '../social/domain/activity';
import type { StudioDependencies } from './app';
import { ConfirmationDialog } from './confirmation-dialog';
import { useOwnedOperation } from './use-owned-operation';
export function ProfileBlocking({ profile, dependencies: d, onBusyChange }: { profile: Pick<Profile, 'id' | 'username' | 'name' | 'relationship'>; dependencies: StudioDependencies; onBusyChange?: (busy: boolean) => void }) {
  const client = useQueryClient(), operation = useOwnedOperation(d);
  const [commands] = useState(() => blockingCommands(d.blocking, d.operationId));
  const [selected, setSelected] = useState<BlockReview | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(false), [stale, setStale] = useState(false), [completed, setCompleted] = useState(false);
  useEffect(() => { onBusyChange?.(busy); }, [busy, onBusyChange]);
  const admitted = useRef(false);
  useEffect(() => () => commands.dispose(), [commands]);
  useBlocker({ shouldBlockFn: () => admitted.current, enableBeforeUnload: busy });
  async function review() {
    if (admitted.current) return;
    admitted.current = true; setBusy(true); setError(false); setStale(false); setSelected(null); commands.clear();
    try {
      const result = await operation.run(signal => d.blocking.review({ userId: profile.id, username: profile.username, displayName: profile.name }, signal));
      if (operation.active()) setSelected(result);
    } catch { if (operation.active()) setError(true); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  async function confirm() {
    if (!selected || admitted.current || stale) return;
    admitted.current = true; setBusy(true); setError(false);
    try {
      const result = await operation.run(signal => commands.submit(selected, signal));
      if (!result || !operation.active()) return;
      await client.cancelQueries({ predicate: query => query.queryKey[0] === d.accountScope });
      if (!operation.active()) return;
      client.removeQueries({ predicate: query => query.queryKey[0] === d.accountScope });
      void d.notifications.invalidate();
      setCompleted(true);
    } catch (failure) {
      if (operation.active()) {
        setError(true);
        if (failure && typeof failure === 'object' && 'status' in failure && [400, 404, 409].includes(Number(failure.status))) setStale(true);
      }
    } finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  if (completed) return <Navigate to="/settings/$section" params={{ section: 'blocked' }} replace />;
  if (profile.relationship === 'self') return null;
  return <div className="studio-profile-blocking">
    <button disabled={busy} aria-label={d.i18n.t('blocking.blockActionLabel', { username: profile.username })} onClick={() => void review()}>{d.i18n.t(busy ? 'blocking.reviewing' : 'blocking.blockAction')}</button>
    {error && !selected && <p role="alert">{d.i18n.t('blocking.operationFailed')}</p>}
    {selected && <ConfirmationDialog title={d.i18n.t('blocking.confirmTitle', { username: selected.target.username })} busy={busy} confirmDisabled={stale} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t(busy ? 'blocking.blocking' : 'blocking.blockAction')} cancel={() => { setSelected(null); setError(false); commands.clear(); }} confirm={() => void confirm()}>
      <strong>{selected.target.displayName}</strong><p>{d.i18n.t('blocking.confirmDescription')}</p>
      {!!selected.sharedPaths.length && <><p>{d.i18n.t('blocking.sharedPathsWarning', { username: selected.target.username, count: selected.sharedPaths.length, paths: selected.sharedPaths.map(path => path.name).join(', ') })}</p><p>{d.i18n.t('blocking.leavePathsSeparately')}</p><ul>{selected.sharedPaths.map(path => <li key={path.id}><Link to="/paths/$pathId" params={{ pathId: path.id }}>{path.name}</Link></li>)}</ul></>}
      {error && <div role="alert"><p>{d.i18n.t('blocking.operationFailed')}</p><button disabled={busy} onClick={() => void review()}>{d.i18n.t('common.refresh')}</button></div>}
    </ConfirmationDialog>}
  </div>;
}
