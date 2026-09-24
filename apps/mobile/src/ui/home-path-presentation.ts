import { intervalProgress, type IntervalProgress, type IntervalProgressProjection } from '@hourpaths/client-core';

export function homeIntervalProgress(
  authoritative: IntervalProgressProjection | undefined,
): IntervalProgress | undefined {
  return intervalProgress(authoritative);
}
