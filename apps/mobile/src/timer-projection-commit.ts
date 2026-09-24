export function commitTimerProjectionBeforeRender<T>(
  next: T,
  advanceAuthoritativeProjection: (value: T) => void,
  enqueueRender: (value: T) => void,
): void {
  advanceAuthoritativeProjection(next);
  enqueueRender(next);
}
