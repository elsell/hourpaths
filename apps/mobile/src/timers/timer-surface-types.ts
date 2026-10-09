import type { NativeTimerSurfacePort, RunningTimerSurface } from '@hourpaths/client-core';

export type TimerSurfaceCopy = Readonly<{
  channel: () => string;
  title: (count: number) => string;
  remaining: (count: number) => string;
  count: (count: number) => string;
}>;

export type TimerSurfacePresentation = {
  title: string;
  count: string;
  remaining: string;
  timers: RunningTimerSurface[];
};

export type CreateTimerSurface = (copy: TimerSurfaceCopy) => NativeTimerSurfacePort;
