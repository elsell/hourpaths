import { useEffect, useState, type ReactNode } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { EnforcementSettings } from './enforcement-settings';
import { AccountExport } from './account-export';

export function AccountRestrictionBoundary({ dependencies: d, children }: { dependencies: StudioDependencies; children: ReactNode }) {
  const controller = d.accountRestriction, client = useQueryClient(), location = useLocation();
  const [state, setState] = useState(controller?.state);
  const [actionFailed, setActionFailed] = useState(false);
  useEffect(() => {
    if (!controller) return;
    let required = controller.state.required;
    const stop = controller.subscribe(() => {
      const next = controller.state; setState(next);
      if (next.required && !required) void client.cancelQueries();
      if (!next.required && required) { d.offline?.retry(); void client.invalidateQueries(); }
      required = next.required;
    });
    void controller.refresh();
    return stop;
  }, [controller, client, d]);
  useEffect(() => {
    const until = state?.review?.restriction?.until;
    if (!controller || !until || !state.required) return;
    const delay = Date.parse(until) - d.now();
    if (delay <= 0) return; // Only the server can lift a known restriction.
    const timer = setTimeout(() => void controller.refresh(), Math.min(delay + 100, 2_147_483_647));
    return () => clearTimeout(timer);
  }, [controller, state?.review, d]);
  if (!controller || !state?.required || location.pathname.endsWith('/delete-account')) return children;
  return <main className="studio studio-entry"><section className="studio-entry-card">
    <h1>{d.i18n.t('accountRestriction.heading')}</h1><p>{d.i18n.t('accountRestriction.body')}</p>
    {(state.error || actionFailed) && <p role="alert">{d.i18n.t('errors.temporarilyUnavailable')}</p>}
    <button disabled={state.loading} onClick={() => void controller.refresh()}>{d.i18n.t('common.refresh')}</button>
    <EnforcementSettings dependencies={d} />
    {d.policyReview?.exportData && <AccountExport controller={d.policyReview.exportData} i18n={d.i18n} />}
    <div className="studio-settings-actions">
      <button disabled={['collecting', 'saving'].includes(d.policyReview?.exportData?.state.phase ?? '')} onClick={() => { try { d.session.keepRunning(); } catch { setActionFailed(true); } }}>{d.i18n.t('auth.signOut')}</button>
      {d.deletion && !['collecting', 'saving'].includes(d.policyReview?.exportData?.state.phase ?? '') && <Link to="/delete-account">{d.i18n.t('accountDelete.heading')}</Link>}
    </div>
  </section></main>;
}
