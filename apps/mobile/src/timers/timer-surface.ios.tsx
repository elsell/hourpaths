import { createLiveActivity } from 'expo-widgets';
import { Platform } from 'react-native';
import type { CreateTimerSurface, TimerSurfacePresentation } from './timer-surface-types';
import { timerLayout } from './timer-layout.ios';

// Lazy construction keeps unsupported/denied native presentation inside the
// coordinator's failure boundary, separate from durable timer commands.
let activity: ReturnType<typeof createLiveActivity<TimerSurfacePresentation>> | undefined;
function factory() {
  return activity ??= createLiveActivity<TimerSurfacePresentation>('HourPathsTimers', timerLayout);
}

export const createTimerSurface: CreateTimerSurface = copy => ({
  async clear() {
    if (Number.parseFloat(String(Platform.Version)) < 16.1) return;
    for (const instance of factory().getInstances()) await instance.end('immediate');
  },
  async replace(timers) {
    if (Number.parseFloat(String(Platform.Version)) < 16.2) return;
    if (!timers.length) {
      for (const instance of factory().getInstances()) await instance.end('immediate');
      return;
    }
    const ordered = [...timers].sort((a, b) => a.startedAt - b.startedAt || a.id.localeCompare(b.id));
    const props: TimerSurfacePresentation = {
      title: copy.title(ordered.length), count: copy.count(ordered.length),
      remaining: ordered.length > 3 ? copy.remaining(ordered.length - 3) : '',
      timers: ordered.slice(0, 3),
    };
    const [current, ...duplicates] = factory().getInstances();
    for (const duplicate of duplicates) await duplicate.end('immediate');
    if (current) await current.update(props);
    else factory().start(props, 'hourpaths:///home');
  },
});
