import type { SessionPath, TimerState } from '@hourpaths/api-client';
import { NativeTimerSurfaceCoordinator } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { homePathSections } from '../ui/home-path-sections';
import { createTimerSurface } from './timer-surface';

export function nativeTimerSurfaces(i18n: Translator) {
  const coordinator = new NativeTimerSurfaceCoordinator(createTimerSurface({
    channel: () => i18n.t('nativeTimers.channel'),
    title: count => i18n.t('nativeTimers.running', { count }),
    remaining: count => i18n.t('nativeTimers.remaining', { count }),
    count: count => i18n.number(count),
  }));
  return {
    clear: () => coordinator.account(null),
    publish(owner: string, paths: readonly SessionPath[], timers: Readonly<Record<string, TimerState | undefined>>) {
      void coordinator.account(owner);
      return coordinator.publish(owner, homePathSections(paths, timers).active.map(path => {
        const timer = timers[path.id]!.timer!;
        return { id: timer.id, pathId: path.id, name: path.name, startedAt: Date.parse(timer.startedAt) };
      }));
    },
  };
}
