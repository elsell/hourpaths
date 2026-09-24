export function boundedAccessibilityProgress(targetValue: number, visualValue: number) {
  const max = Number.isFinite(targetValue) && targetValue > 0 ? targetValue : 0;
  const now = Number.isFinite(visualValue)
    ? Math.min(max, Math.max(0, visualValue))
    : 0;
  return { max, now };
}
