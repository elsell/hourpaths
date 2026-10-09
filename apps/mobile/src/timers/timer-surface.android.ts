import { requireNativeModule } from 'expo';
import type { CreateTimerSurface } from './timer-surface-types';

type TimerModule = {
  clear(): Promise<void>;
  replace(title: string, names: string[], startedAt: number, channelName: string): Promise<void>;
};
const native = () => requireNativeModule<TimerModule>('HourPathsTimers');

export const createTimerSurface: CreateTimerSurface = copy => ({
  clear: () => native().clear(),
  async replace(timers) {
    if (!timers.length) return native().clear();
    const ordered = [...timers].sort((a, b) => a.startedAt - b.startedAt || a.id.localeCompare(b.id));
    const names = ordered.slice(0, 6).map(timer => timer.name);
    if (ordered.length > 6) names.push(copy.remaining(ordered.length - 6));
    await native().replace(copy.title(ordered.length), names, ordered[0].startedAt, copy.channel());
  },
});
