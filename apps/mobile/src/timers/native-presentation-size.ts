import type { TimerSurfacePresentation } from './timer-surface-types';

// Internal ActivityKit registration identifier, not presentation copy.
export const nativeTimerRegistration = {
  activityIdentifier: 'HourPathsTimers',
} as const;

/** ActivityKit limits encoded content state, including the serialized props
 * string and its escaping. Keep headroom for stopped timestamps/native updates. */
export function nativePresentationBytes(props: TimerSurfacePresentation): number {
  const encoded = JSON.stringify({ name: nativeTimerRegistration.activityIdentifier, props: JSON.stringify(props) });
  let size = 0;
  for (const character of encoded) {
    const value = character.codePointAt(0)!;
    size += value <= 0x7f ? 1 : value <= 0x7ff ? 2 : value <= 0xffff ? 3 : 4;
  }
  return size;
}
export function fitNativePresentation(props: TimerSurfacePresentation, total: number, remaining: (count: number) => string): void {
  while (props.timers.length > 1 && nativePresentationBytes(props) > 3400) {
    props.timers.pop();
    props.remaining = remaining(total - props.timers.length);
  }
  if (nativePresentationBytes(props) > 3400) throw new Error('native_timer_presentation_too_large');
}
