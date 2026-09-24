import type { ManualActivityFormState } from '@hourpaths/client-core';

export type ManualActivityDraft = Readonly<{
  durationSeconds: string;
  localDate: string;
  localTime: string;
  note: string;
}>;

function normalizedDuration(value: string): string {
  const trimmed = value.trim();
  return /^\d+$/.test(trimmed) ? trimmed.replace(/^0+(?=\d)/, '') : trimmed;
}

export function manualActivityDraft(form: ManualActivityFormState, note: string): ManualActivityDraft {
  return {
    durationSeconds: normalizedDuration(form.durationSeconds),
    localDate: form.localDate,
    localTime: form.localTime,
    note,
  };
}

export function manualActivityDraftChanged(
  baseline: ManualActivityDraft,
  current: ManualActivityDraft,
): boolean {
  return baseline.durationSeconds !== current.durationSeconds || baseline.localDate !== current.localDate ||
    baseline.localTime !== current.localTime || baseline.note !== current.note;
}
