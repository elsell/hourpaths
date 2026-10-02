import { useState } from 'react';
import { createManualActivityFormState, durationParts, manualActivityParticipantNow, overrideManualActivityOccurrence, reviewedManualActivityInterval, secondsFromDurationParts, updateManualActivityDuration } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import type { ClockCorrectionView, OfflineStatus } from '../offline/ports/tracking-status';
import { ActivityTimingFields } from './activity-timing-fields';

export function ClockCorrection({ correction, service, i18n, now, close }: {
  correction: ClockCorrectionView; service: OfflineStatus; i18n: Translator; now(): number; close(): void;
}) {
  const [form, setForm] = useState(() => ({ ...createManualActivityFormState(manualActivityParticipantNow(correction.reviewedStartedAt ?? correction.startedAt, correction.timeZone)), durationSeconds: correction.reviewedStartedAt ? String(Math.floor((Date.parse(correction.endedAt) - Date.parse(correction.reviewedStartedAt)) / 1000)) : '', occurrenceTouched: true }));
  const [parts, setParts] = useState(() => durationParts(form.durationSeconds));
  const [busy, setBusy] = useState(false), [error, setError] = useState<'invalid' | 'request' | null>(null);
  const clock = () => manualActivityParticipantNow(new Date(now()).toISOString(), correction.timeZone);
  async function submit() {
    if (busy) return;
    let interval: { startedAt: string; endedAt: string };
    try { interval = reviewedManualActivityInterval(form, clock()); }
    catch { setError('invalid'); return; }
    setBusy(true); setError(null);
    try { await service.correct(correction.id, interval.startedAt, interval.endedAt); close(); }
    catch { setError('request'); }
    finally { setBusy(false); }
  }
  return <form className="studio-form studio-activity-form" onSubmit={event => { event.preventDefault(); void submit(); }}>
    <h2>{i18n.t('offline.correctHeading')}</h2><strong>{correction.pathName}</strong>
    <p>{i18n.t('offline.correctBody')}</p><p>{i18n.t('activity.timeZone', { timeZone: correction.timeZone })}</p>
    <fieldset disabled={busy}>
      <ActivityTimingFields i18n={i18n} form={form} parts={parts}
        onOccurrence={patch => setForm(overrideManualActivityOccurrence(form, patch))}
        onParts={next => { setParts(next); setForm(updateManualActivityDuration(form, secondsFromDurationParts(next), clock())); }} />
      <div className="studio-form-actions"><button type="button" onClick={close}>{i18n.t('common.cancel')}</button><button type="submit">{i18n.t(busy ? 'activity.saving' : 'activity.save')}</button></div>
    </fieldset>
    {error && <p role="alert">{i18n.t(error === 'invalid' ? 'activity.invalid' : 'errors.temporarilyUnavailable')}</p>}
  </form>;
}
