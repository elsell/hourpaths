import type { Translator } from '@hourpaths/i18n';
import type { ManualActivityFormState, ManualActivityLocalDateTime } from '@hourpaths/client-core';

type Parts = { hours: string; minutes: string; seconds: string };
/** Shared date, time and duration controls for manual entries and corrections. */
export function ActivityTimingFields({ i18n, form, parts, onOccurrence, onParts }: {
  i18n: Translator; form: ManualActivityFormState; parts: Parts;
  onOccurrence(patch: Partial<ManualActivityLocalDateTime>): void; onParts(parts: Parts): void;
}) {
  return <>
    <div className="studio-form-pair"><label>{i18n.t('activity.date')}<input autoFocus required type="date" value={form.localDate} onChange={event => onOccurrence({ localDate: event.target.value })} /></label>
      <label>{i18n.t('activity.startTime')}<input required type="time" step="1" value={form.localTime} onChange={event => { const value = event.target.value; onOccurrence({ localTime: value.length === 5 ? `${value}:00` : value }); }} /></label></div>
    <fieldset className="studio-duration-parts"><legend>{i18n.t('activity.durationValue')}</legend>{(['hours', 'minutes', 'seconds'] as const).map(unit => <label key={unit}>{i18n.t(`activity.duration.${unit}`)}<input type="number" min="0" step="1" inputMode="numeric" value={parts[unit]} onChange={event => onParts({ ...parts, [unit]: event.target.value })} /></label>)}</fieldset>
  </>;
}
