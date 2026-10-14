import { retainStoppedTimers } from './retain-stopped-timers';
import { fitNativePresentation, nativeTimerRegistration } from './native-presentation-size';
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
  return activity ??= createLiveActivity<TimerSurfacePresentation>(nativeTimerRegistration.activityIdentifier, timerLayout);
}

export const createTimerSurface: CreateTimerSurface = copy => ({
  subscribe(changed) {
    const listener = addUserInteractionListener(() => {
      changed();
    });
    return () => listener.remove();
  },
  async revokeActions() { await (await openNativeOfflineStorage()).nativeLease.clear(); },
  async clear() {
    await (await openNativeOfflineStorage()).nativeLease.clear();
    if (Number.parseFloat(String(Platform.Version)) < 16.1) return;
    for (const instance of factory().getInstances()) await instance.end('immediate');
  },
  async replace(timers, account) {
    if (Number.parseFloat(String(Platform.Version)) < 16.2) return;
    const storage = await openNativeOfflineStorage();
    const lease = account ? await storage.nativeLease.bind(account.owner, account.expiresAt) : undefined;
    if (!account) await storage.nativeLease.clear();
    const snapshot = account ? await storage.tracking.read(account.owner) : undefined;
    let ordered = [...timers].sort((a, b) => a.startedAt - b.startedAt || a.id.localeCompare(b.id));
    if (AppState.currentState !== 'active' && lease && snapshot) {
      const stored = await storage.nativeLease.readPresentation(lease.owner, lease.generation);
      const previous: TimerSurfacePresentation | undefined = stored ? JSON.parse(stored) : undefined;
      if (previous?.owner === lease.owner && previous.accountLease === lease.generation) {
        ordered = retainStoppedTimers(ordered, previous.timers,
          snapshot.paths.filter(path => !snapshot.unavailablePaths?.includes(path.id)).map(path => path.id));
      }
    }
    if (!ordered.length) {
      await storage.nativeLease.clear();
      for (const instance of factory().getInstances()) await instance.end('immediate');
      return;
    }
    const props: TimerSurfacePresentation = {
      title: copy.title(timers.length), count: copy.count(timers.length),
      remaining: ordered.length > 2 ? copy.remaining(ordered.length - 2) : '',
      timers: ordered.slice(0, 2),
    };
    if (lease && snapshot && Number.parseFloat(String(Platform.Version)) >= 17) {
      props.owner = lease.owner;
      props.accountLease = lease.generation;
      props.startLabel = copy.start(); props.stopLabel = copy.stop();
      props.titles = Object.fromEntries(Array.from({ length: props.timers.length + 1 }, (_, offset) => {
        const stopped = props.timers.filter(timer => timer.stoppedAt !== undefined).length;
        const count = Math.max(0, snapshot.timers.length - (props.timers.length - stopped) + offset);
        return [String(count), copy.title(count)];
      }));
      props.timers = props.timers.map(timer => {
        const active = snapshot.timers.find(active => active.id === timer.id && active.pathId === timer.pathId);
        return { ...timer, action: {
          label: active ? copy.stop() : copy.start(), target: JSON.stringify({ lease: lease.generation, action: {
            owner: lease.owner, revision: snapshot.revision, pathId: timer.pathId,
            timerId: active?.id ?? randomUUID(), operationId: randomUUID(), kind: active ? 'stop' : 'start',
          } }),
        } };
      });
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
