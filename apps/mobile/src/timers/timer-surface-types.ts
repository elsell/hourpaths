import type { NativeTimerSurfacePort, RunningTimerSurface } from '@hourpaths/client-core';

export type TimerSurfaceCopy = Readonly<{
  start: () => string;
  stop: () => string;
  channel: () => string;
  title: (count: number) => string;
  remaining: (count: number) => string;
  count: (count: number) => string;
}>;

export type TimerSurfacePresentation = {
  accountLease?: string;
  owner?: string;
  startLabel?: string;
  stopLabel?: string;
  titles?: Record<string, string>;
  title: string;
  count: string;
  remaining: string;
  timers: RunningTimerSurface[];
};

export type CreateTimerSurface = (copy: TimerSurfaceCopy) => NativeTimerSurfacePort;
