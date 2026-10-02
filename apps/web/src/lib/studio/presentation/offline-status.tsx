import { useState } from 'react';
import { ClockCorrection } from './clock-correction';
import { useMutation, useQuery } from '@tanstack/react-query';
import type { Translator } from '@hourpaths/i18n';
import type { OfflineStatus } from '../offline/ports/tracking-status';
import { duration } from './duration';

export function OfflineStatusPanel({ service, accountScope, i18n, now }: { service: OfflineStatus; accountScope: string; i18n: Translator; now(): number }) {
  const [reviewing, setReviewing] = useState<string | null>(null);
  const query = useQuery({ queryKey: [accountScope, 'offlineState'], queryFn: () => service.snapshot() });
  const dismiss = useMutation({ mutationFn: (id: string) => service.dismissNotice(id) });
  const state = query.data;
  if (!state || !state.showBanner && !state.pending && !state.notices.length && !state.corrections.length) return null;
  return <section className="studio studio-sync-status" aria-label={i18n.t('offline.status')}>
    {state.showBanner && <div className="studio-sync-message"><p role="status">{i18n.t('offline.banner')}</p>
      <button onClick={() => service.dismissBanner()}>{i18n.t('common.dismiss')}</button></div>}
    {state.pending && <div className="studio-sync-message"><p role="status">{i18n.t('offline.pending')}</p>
      <button onClick={() => service.retry()}>{i18n.t('common.retry')}</button></div>}
    {state.corrections.map(correction => <div key={correction.id}>
      {reviewing === correction.id ? <ClockCorrection correction={correction} service={service} i18n={i18n} now={now} close={() => setReviewing(null)} /> : <div className="studio-sync-message">
        <p>{i18n.t('offline.correctionForPath', { path: correction.pathName || i18n.t('offline.retainedPath') })}</p>
        <button onClick={() => setReviewing(correction.id)}>{i18n.t('offline.review')}</button>
      </div>}
    </div>)}
    {state.notices.map(notice => <div className="studio-sync-message" key={notice.id}>
      <p>{notice.pathName && <strong>{notice.pathName}<br /></strong>}{i18n.t(notice.subject === 'activity' && notice.reason === 'archived' ? 'offline.activityArchived' : notice.reason === 'subsecond' ? 'timer.subsecondNotice' : `offline.rejection.${notice.reason}`)}
        {notice.reason === 'archived' && notice.subject !== 'activity' && <span> {i18n.t('offline.archiveAmounts', {
          saved: duration(i18n, notice.savedSeconds ?? 0), discarded: duration(i18n, notice.discardedSeconds ?? 0),
        })}</span>}</p>
      <button disabled={dismiss.isPending} onClick={() => dismiss.mutate(notice.id)}>{i18n.t('common.dismiss')}</button>
    </div>)}
    {dismiss.isError && <p role="alert">{i18n.t('errors.temporarilyUnavailable')}</p>}
  </section>;
}
