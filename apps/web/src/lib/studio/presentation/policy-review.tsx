import { AccountExport } from './account-export';
import { PolicyTimers } from './policy-timers';
import { useEffect, useState, type ReactNode } from 'react';
import { Link, useLocation } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import type { Translator } from '@hourpaths/i18n';
import type { PolicyReviewController } from '@hourpaths/client-core';
import type { StudioDependencies } from './app';

export function PolicyReviewBoundary({ dependencies: d, children }: { dependencies: StudioDependencies; children: ReactNode }) {
  const controller = d.policyReview;
  const [signOutFailed, setSignOutFailed] = useState(false);
  const [state, setState] = useState(controller?.state);
  const location = useLocation();
  const client = useQueryClient();
  useEffect(() => {
    if (!controller) return;
    let previouslyRequired = controller.state.required;
    const stop = controller.subscribe(() => {
      const next = controller.state; setState(next);
      if (next.required && !previouslyRequired) void client.cancelQueries();
      if (!next.required && previouslyRequired) { d.offline?.retry(); void client.invalidateQueries(); }
      previouslyRequired = next.required;
    });
    void controller.refresh();
    return stop;
  }, [controller, client, d]);
  if (!controller || !state?.required || location.pathname.endsWith('/delete-account')) return children;
  return <main className="studio studio-entry"><section className="studio-entry-card">
    <PolicyReviewForm controller={controller} i18n={d.i18n} />
    {controller.timers && state.review && <PolicyTimers controller={controller.timers} i18n={d.i18n} />}
    {controller.exportData && state.review && <AccountExport controller={controller.exportData} i18n={d.i18n} />}
    <section aria-label={d.i18n.t('policyReview.accountOptions')}>
      {signOutFailed && <p role="alert">{d.i18n.t('errors.temporarilyUnavailable')}</p>}
      <button type="button" disabled={state.saving || ['collecting', 'saving'].includes(controller.exportData?.state.phase ?? '')} onClick={() => { try { d.session.keepRunning(); } catch { setSignOutFailed(true); } }}>
        {d.i18n.t('settings.account.activeTimers.keepRunningAndSignOut')}
      </button>
      {d.deletion && !['collecting', 'saving'].includes(controller.exportData?.state.phase ?? '') && <Link to="/delete-account">{d.i18n.t('accountDelete.heading')}</Link>}
    </section>
  </section></main>;
}

export function PolicyReviewForm({ controller, i18n }: { controller: PolicyReviewController; i18n: Translator }) {
  const [state, setState] = useState(controller.state);
  const [confirmed, setConfirmed] = useState({ termsAccepted: false, privacyAcknowledged: false, guidelinesAccepted: false });
  useEffect(() => { setState(controller.state); return controller.subscribe(() => setState(controller.state)); }, [controller]);
  useEffect(() => { setConfirmed({ termsAccepted: false, privacyAcknowledged: false, guidelinesAccepted: false }); }, [state.review?.token]);
  const busy = state.loading || state.saving || ['collecting', 'saving'].includes(controller.exportData?.state.phase ?? '');
  const review = state.review;
  return <form className="studio-entry-form" onSubmit={event => { event.preventDefault(); void controller.accept(confirmed); }}>
    <h1>{i18n.t('policyReview.heading')}</h1><p>{i18n.t('policyReview.body')}</p>
    {review && <fieldset disabled={busy} className="studio-entry-consents">
      
      <div><label><input type="checkbox" required checked={confirmed.termsAccepted} onChange={event => setConfirmed(value => ({ ...value, termsAccepted: event.target.checked }))} />{i18n.t('onboarding.termsAcceptance')}</label><button type="button" className="studio-entry-link" onClick={() => void controller.openPolicy('terms')}>{i18n.t('onboarding.termsLink', { version: review.policies.terms.version })}</button></div>
      <div><label><input type="checkbox" required checked={confirmed.privacyAcknowledged} onChange={event => setConfirmed(value => ({ ...value, privacyAcknowledged: event.target.checked }))} />{i18n.t('onboarding.privacyAcknowledgement')}</label><button type="button" className="studio-entry-link" onClick={() => void controller.openPolicy('privacy')}>{i18n.t('onboarding.privacyLink', { version: review.policies.privacy.version })}</button></div>
      <div><label><input type="checkbox" required checked={confirmed.guidelinesAccepted} onChange={event => setConfirmed(value => ({ ...value, guidelinesAccepted: event.target.checked }))} />{i18n.t('onboarding.guidelinesAcceptance')}</label><button type="button" className="studio-entry-link" onClick={() => void controller.openPolicy('guidelines')}>{i18n.t('onboarding.guidelinesLink', { version: review.policies.guidelines.version })}</button></div>
    </fieldset>}
    {state.error && <p role="alert">{i18n.t(state.error === 'review_changed' ? 'policyReview.changed' : state.error === 'invalid' ? 'policyReview.invalid' : 'policyReview.unavailable')}</p>}
    {state.loading && <p role="status">{i18n.t('common.loading')}</p>}
    {state.error && <button type="button" disabled={busy} onClick={() => void controller.refresh()}>{i18n.t('common.retry')}</button>}
    <button className="studio-primary studio-entry-primary" disabled={busy || !review || !Object.values(confirmed).every(Boolean)} type="submit">{i18n.t('policyReview.continue')}</button>
  </form>;
}
