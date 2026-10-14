import { fitNativePresentation } from './native-presentation-size';
import { requireNativeModule } from 'expo';
import { randomUUID } from 'expo-crypto';
import { openNativeOfflineStorage } from '../offline/native-tracking-store';
import { addUserInteractionListener, createLiveActivity } from 'expo-widgets';
import { AppState, Platform } from 'react-native';
import type { CreateTimerSurface, TimerSurfacePresentation } from './timer-surface-types';
import { timerLayout } from './ios-timer-layout';

// Lazy construction keeps unsupported/denied native presentation inside the
// coordinator's failure boundary, separate from durable timer commands.
let activity: ReturnType<typeof createLiveActivity<TimerSurfacePresentation>> | undefined;
function factory() {
  return activity ??= createLiveActivity<TimerSurfacePresentation>('HourPathsTimers', timerLayout);
}

export const createTimerSurface: CreateTimerSurface = copy => ({
  subscribe(changed) {
    const listener = addUserInteractionListener(() => {
      if (AppState.currentState === 'active') changed();
    });
    return () => listener.remove();
  },
  async clear() {
    await (await openNativeOfflineStorage()).nativeLease.clear();
    if (Number.parseFloat(String(Platform.Version)) < 16.1) return;
    for (const instance of factory().getInstances()) await instance.end('immediate');
  },
  async replace(timers, account) {
    if (Number.parseFloat(String(Platform.Version)) < 16.2) return;
    if (!timers.length) {
      await (await openNativeOfflineStorage()).nativeLease.clear();
      for (const instance of factory().getInstances()) await instance.end('immediate');
      return;
    }
    const storage = await openNativeOfflineStorage();
    const lease = account ? await storage.nativeLease.bind(account.owner, account.expiresAt) : undefined;
    if (!account) await storage.nativeLease.clear();
    const snapshot = account ? await storage.tracking.read(account.owner) : undefined;
    const ordered = [...timers].sort((a, b) => a.startedAt - b.startedAt || a.id.localeCompare(b.id));
    const props: TimerSurfacePresentation = {
      title: copy.title(ordered.length), count: copy.count(ordered.length),
      remaining: ordered.length > 2 ? copy.remaining(ordered.length - 2) : '',
      timers: ordered.slice(0, 2),
    };
    if (lease && snapshot && Number.parseFloat(String(Platform.Version)) >= 17) {
      props.owner = lease.owner;
      props.accountLease = lease.generation;
      props.startLabel = copy.start(); props.stopLabel = copy.stop();
      props.titles = Object.fromEntries(Array.from({ length: Math.min(props.timers.length, snapshot.timers.length) + 1 }, (_, offset) => {
        const count = snapshot.timers.length - offset; return [String(count), copy.title(count)];
      }));
      props.timers = props.timers.map(timer => ({ ...timer,
        ...(snapshot.timers.some(active => active.id === timer.id && active.pathId === timer.pathId) ? { action: {
          label: copy.stop(), target: JSON.stringify({ lease: lease.generation, action: {
            owner: lease.owner, revision: snapshot.revision, pathId: timer.pathId,
            timerId: timer.id, operationId: randomUUID(), kind: 'stop',
          } }),
        } } : {}),
      }));
      fitNativePresentation(props, ordered.length, copy.remaining);
      await requireNativeModule<{ configureActions(path: string, presentation: string): Promise<void> }>('HourPathsTimers')
        .configureActions(storage.databasePath, JSON.stringify(props));
    }
    fitNativePresentation(props, ordered.length, copy.remaining);
    const [current, ...duplicates] = factory().getInstances();
    for (const duplicate of duplicates) await duplicate.end('immediate');
    if (current) await current.update(props);
    else factory().start(props, 'hourpaths:///home');
  },
});
