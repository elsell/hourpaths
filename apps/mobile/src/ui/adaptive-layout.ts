const compactPhoneWidth = 375;
const accessibilityTextScale = 1.3;

export function needsCompactVerticalLayout(width: number, fontScale: number): boolean {
  return width < compactPhoneWidth || fontScale >= accessibilityTextScale;
}
