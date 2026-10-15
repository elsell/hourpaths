import type { LiveActivityRefreshPort, NativeTimerSurfacePort, RunningTimerSurface } from '@hourpaths/client-core';

export type TimerSurfaceCopy = Readonly<{
  start: () => string;
  stop: () => string;
  channel: () => string;
  title: (count: number) => string;
  remaining: (count: number) => string;
  count: (count: number) => string;
}>;

export type TimerSurfaceEnvelope = { owner: string; accountLease: string; localPresentation: true; staleAt: number; presentationRevision?: string };

export type TimerSurfacePresentation = {
  controlAppearance?: { startBackground: string; startForeground: string; stopBackground: string; stopForeground: string };
  activityExpiresAt?: number;
  trackingRevision?: number;
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

export type CreateTimerSurface = (copy: TimerSurfaceCopy, refresh?: LiveActivityRefreshPort) => NativeTimerSurfacePort;
