export type PathShareDraft = Readonly<{ role: 'participant' | 'supporter'; username: string }>;

export function manualShareDraftChanged(baseline: PathShareDraft, current: PathShareDraft): boolean {
  return baseline.role !== current.role || baseline.username.trim() !== current.username.trim();
}
