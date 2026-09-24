export function ownsBlockedAccountLoad(
  loadRevision: number,
  currentRevision: number,
  mutationActive: boolean,
): boolean {
  return !mutationActive && loadRevision === currentRevision;
}
