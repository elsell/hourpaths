import { useState } from 'react';
import { createManualActivityFormState, manualActivityParticipantNow, overrideManualActivityOccurrence, reviewedManualActivityInterval, updateManualActivityDuration, type TrackingSnapshot } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { ManualActivityForm } from './manual-activity-form';

export function OfflineClockCorrection({ correction, i18n, now, save, close }: {
  correction: TrackingSnapshot['corrections'][number]; i18n: Translator; now(): number;
  save(id: string, start: string, end: string): Promise<void>; close(): void;
}) {
  const zone = correction.timer.timeZone;
  const [form, setForm] = useState(() => ({ ...createManualActivityFormState(manualActivityParticipantNow(correction.reviewedStartedAt ?? correction.timer.startedAt, zone)), durationSeconds: correction.reviewedStartedAt ? String(Math.floor((Date.parse(correction.endedAt) - Date.parse(correction.reviewedStartedAt)) / 1000)) : '', occurrenceTouched: true }));
  const [busy, setBusy] = useState(false), [error, setError] = useState<string>();
  const clock = () => manualActivityParticipantNow(new Date(now()).toISOString(), zone);
  async function submit() {
    if (busy) return;
    let interval: { startedAt: string; endedAt: string };
    try { interval = reviewedManualActivityInterval(form, clock()); }
    catch { setError(i18n.t('activity.invalid')); return; }
    setBusy(true); setError(undefined);
    try { await save(correction.timer.id, interval.startedAt, interval.endedAt); close(); }
    catch { setError(i18n.t('errors.temporarilyUnavailable')); }
    finally { setBusy(false); }
  }
  return <ManualActivityForm correction editing={false} busy={busy} form={form} note="" timeZone={zone} errorText={error}
    onCancel={() => { if (!busy) close(); }} onChangeNote={() => {}}
    onChangeOccurrence={patch => setForm(overrideManualActivityOccurrence(form, patch))}
    onChangeDuration={value => setForm(updateManualActivityDuration(form, value, clock()))} onSave={() => void submit()} />;
}
