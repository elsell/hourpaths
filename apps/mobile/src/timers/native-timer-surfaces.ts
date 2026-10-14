import type { SessionPath, TimerState } from '@hourpaths/api-client';
import { NativeTimerSurfaceCoordinator, defaultPathAppearance, pathPalette, intervalProgress, nativeGoalClock, liveTimerProgress, type PathAppearance } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { formatCompactDuration } from '../ui/compact-duration';
import { homePathSections } from '../ui/home-path-sections';
import { createTimerSurface } from './timer-surface';

export function nativeTimerSurfaces(i18n: Translator, now: () => number = Date.now) {
  const port = createTimerSurface({
    start: () => i18n.t('timer.start'),
    stop: () => i18n.t('timer.stop'),
    channel: () => i18n.t('nativeTimers.channel'),
    title: count => i18n.t('nativeTimers.running', { count }),
    remaining: count => i18n.t('nativeTimers.remaining', { count }),
    count: count => i18n.number(count),
  });
  const coordinator = new NativeTimerSurfaceCoordinator(port);
  return {
    subscribe: (changed: () => void) => port.subscribe?.(changed) ?? (() => undefined),
    clear: () => coordinator.account(null),
    publish(owner: string, paths: readonly SessionPath[], timers: Readonly<Record<string, TimerState | undefined>>, appearances: Readonly<Record<string, PathAppearance>> = {}, expiresAt?: number) {
      void coordinator.account(owner);
      const sampledAt = now();
      return coordinator.publish(owner, homePathSections(paths, timers).active.map(path => {
        const timer = timers[path.id]!.timer!;
        const appearance = appearances[path.id] ?? defaultPathAppearance(path.id);
        const state = timers[path.id]!;
        const progress = intervalProgress(liveTimerProgress(state, sampledAt).intervalProgress);
        const clock = nativeGoalClock(state.intervalProgress, Date.parse(timer.startedAt), sampledAt);
        return {
          id: timer.id, pathId: path.id, name: path.name, startedAt: Date.parse(timer.startedAt),
          emoji: appearance.emoji, color: pathPalette[appearance.color].background,
          ...(progress && progress.targetSeconds > 0 ? { goal: {
            fraction: progress.visualSeconds / progress.targetSeconds,
            ...clock,
            targetText: i18n.t('nativeTimers.goalTarget', { target: formatCompactDuration(progress.targetSeconds, i18n) }),
            text: i18n.t('path.progress.intervalCompact', {
              accumulated: formatCompactDuration(progress.accumulatedSeconds, i18n),
              target: formatCompactDuration(progress.targetSeconds, i18n),
            }),
          } } : {}),
        };
      }), expiresAt === undefined ? undefined : { owner, expiresAt });
    },
  };
}
