export function normalizedComment(value: string) {
  const normalized = value.trim().normalize('NFC');
  return normalized.length > 0 && [...normalized].length <= 2_000 &&
    !/[\u0000-\u0009\u000b-\u001f\u007f]/u.test(normalized)
    ? normalized
    : undefined;
}

export function commentDraftHasMeaningfulText(value: string) {
  return value.trim().normalize('NFC').length > 0;
}

export function commentDraftIsDirty(baseline: string, draft: string) {
  return baseline.trim().normalize('NFC') !== draft.trim().normalize('NFC');
}

export function commentDraftAfterFailedSubmit(current: string, submitted: string) {
  return current.length === 0 ? submitted : current;
}
