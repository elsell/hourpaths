import { useEffect, useState } from 'react';
import type { PolicyTimersController } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';

export function PolicyTimers({ controller, i18n }: { controller: PolicyTimersController; i18n: Translator }) {
  const [state, setState] = useState(controller.state);
  useEffect(() => {
    setState(controller.state);
    const unsubscribe = controller.subscribe(() => setState(controller.state));
    void controller.refresh();
    return unsubscribe;
  }, [controller]);
  return <section className="studio-retained-timers" aria-label={i18n.t('policyReview.timers')}>
    <h2>{i18n.t('policyReview.timers')}</h2>
    {state.timers.map(timer => <article className="studio-retained-timer" key={timer.source + timer.id}>
      <strong>{timer.name || i18n.t('offline.retainedPath')}</strong>
      <button type="button" aria-label={i18n.t('policyReview.stopTimer', { name: timer.name || i18n.t('offline.retainedPath') })}
        disabled={state.loading || !!state.stopping} onClick={() => void controller.stop(timer.id, timer.source)}>
        {i18n.t('timer.stop')}
      </button>
    </article>)}
    {state.loading && <p role="status">{i18n.t('common.loading')}</p>}
    {!state.loading && !state.error && !state.timers.length && !state.pending && <p>{i18n.t('policyReview.noTimers')}</p>}
    {state.pending && <p role="status">{i18n.t('policyReview.pendingStops')}</p>}
    {state.error && <p role="alert">{i18n.t('errors.temporarilyUnavailable')}</p>}
    <button type="button" disabled={state.loading || !!state.stopping} onClick={() => void controller.refresh()}>{i18n.t(state.error ? 'common.retry' : 'common.refresh')}</button>
    {state.nextCursor && <button type="button" disabled={state.loading || !!state.stopping} onClick={() => void controller.refresh(true)}>{i18n.t('policyReview.moreTimers')}</button>}
  </section>;
}
