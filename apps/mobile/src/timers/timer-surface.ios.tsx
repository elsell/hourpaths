import { LiveActivityRefreshCoordinator, nativeGoalPeriodSchedule, type LiveActivityRefreshIdentity } from '@hourpaths/client-core';
import { retainStoppedTimers } from './retain-stopped-timers';
import { fitNativePresentation, nativeTimerRegistration } from './native-presentation-size';
import { requireNativeModule } from 'expo';
import { randomUUID } from 'expo-crypto';
import { openNativeOfflineStorage } from '../offline/native-tracking-store';
import { addUserInteractionListener, createLiveActivity } from 'expo-widgets';
import { AppState, Platform } from 'react-native';
import type { CreateTimerSurface, TimerSurfaceEnvelope, TimerSurfacePresentation } from './timer-surface-types';
import { timerLayout } from './ios-timer-layout';
import { mobileTheme } from '../ui/tokens';

// Lazy construction keeps unsupported/denied native presentation inside the
// coordinator's failure boundary, separate from durable timer commands.
let activity: ReturnType<typeof createLiveActivity<TimerSurfacePresentation | TimerSurfaceEnvelope>> | undefined;
function factory() {
  return activity ??= createLiveActivity<TimerSurfacePresentation | TimerSurfaceEnvelope>(nativeTimerRegistration.activityIdentifier, timerLayout);
}

async function clearPresentation() {
  if (Number.parseFloat(String(Platform.Version)) >= 17) {
    await requireNativeModule<{ clearPresentation(): Promise<void> }>('HourPathsTimers').clearPresentation();
  }
}

export const createTimerSurface: CreateTimerSurface = (copy, refreshPort) => {
  const refresh = refreshPort ? new LiveActivityRefreshCoordinator(refreshPort, Date.now) : undefined;
  let tokenSubscription: { remove(): void } | undefined;
  let tokenGeneration = 0;
  function clearRefresh(persisted?: LiveActivityRefreshIdentity) {
    ++tokenGeneration;
    tokenSubscription?.remove();
    tokenSubscription = undefined;
    void refresh?.clear(persisted);
  }
  return ({
  subscribe(changed) {
    const listener = addUserInteractionListener(() => {
      changed();
    });
    const foreground = AppState.addEventListener('change', state => { if (state === 'active') changed(); });
    return () => { listener.remove(); foreground.remove(); };
  },
  async revokeActions() { clearRefresh(); clearRefresh(await (await openNativeOfflineStorage()).nativeLease.clear()); await clearPresentation(); },
  async clear() {
    clearRefresh();
    clearRefresh(await (await openNativeOfflineStorage()).nativeLease.clear());
    await clearPresentation();
    if (Number.parseFloat(String(Platform.Version)) < 16.1) return;
    for (const instance of factory().getInstances()) await instance.end('immediate');
  },
  async replace(timers, account) {
    if (Number.parseFloat(String(Platform.Version)) < 16.2) return;
    const storage = await openNativeOfflineStorage();
    let lease = account ? await storage.nativeLease.bind(account.owner, account.expiresAt) : undefined;
    if (!account) { clearRefresh(); clearRefresh(await storage.nativeLease.clear()); await clearPresentation(); }
    const snapshot = account ? await storage.tracking.read(account.owner) : undefined;
    const previousStored = lease ? await storage.nativeLease.readPresentation(lease.owner, lease.generation) : undefined;
    let previous: TimerSurfacePresentation | undefined = previousStored ? JSON.parse(previousStored) : undefined;
    if (previous?.activityExpiresAt && previous.activityExpiresAt <= Date.now()) {
      clearRefresh();
      clearRefresh(await storage.nativeLease.clear());
      await clearPresentation();
      for (const instance of factory().getInstances()) await instance.end('immediate');
      lease = account ? await storage.nativeLease.bind(account.owner, account.expiresAt) : undefined;
      previous = undefined;
    }
    let ordered = [...timers].sort((a, b) => a.startedAt - b.startedAt || a.id.localeCompare(b.id));
    if (AppState.currentState !== 'active' && lease && snapshot) {
      if (previous?.owner === lease.owner && previous.accountLease === lease.generation) {
        ordered = retainStoppedTimers(ordered, previous.timers,
          snapshot.paths.filter(path => !snapshot.unavailablePaths?.includes(path.id)).map(path => path.id));
      }
    }
    if (!ordered.length) {
      clearRefresh();
      clearRefresh(await storage.nativeLease.clear());
      await clearPresentation();
      for (const instance of factory().getInstances()) await instance.end('immediate');
      return;
    }
    const props: TimerSurfacePresentation = {
      controlAppearance: { startBackground: mobileTheme.colors.accent, startForeground: mobileTheme.colors.accentText,
        stopBackground: mobileTheme.colors.errorSurface, stopForeground: mobileTheme.colors.error },
      title: copy.title(timers.length), count: copy.count(timers.length),
      remaining: ordered.length > 2 ? copy.remaining(ordered.length - 2) : '',
      timers: ordered.slice(0, 2),
    };
    let published: TimerSurfacePresentation | TimerSurfaceEnvelope = props;
    if (lease && snapshot && account && Number.parseFloat(String(Platform.Version)) >= 17) {
      const now = Date.now();
      props.activityExpiresAt = Math.min(account.expiresAt, previous?.activityExpiresAt ?? now + 8 * 60 * 60 * 1000);
      props.trackingRevision = snapshot.revision;
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
        return { ...timer, goalPeriods: nativeGoalPeriodSchedule(snapshot, lease.owner, timer.pathId, now, props.activityExpiresAt!), action: {
          label: active ? copy.stop() : copy.start(), target: JSON.stringify({ lease: lease.generation, action: {
            owner: lease.owner, revision: snapshot.revision, pathId: timer.pathId,
            timerId: active?.id ?? randomUUID(), operationId: randomUUID(), kind: active ? 'stop' : 'start',
          } }),
        } };
      });
      const envelope = await requireNativeModule<{ configureActions(path: string, presentation: string): Promise<string> }>('HourPathsTimers')
        .configureActions(storage.databasePath, JSON.stringify(props));
      published = JSON.parse(envelope) as TimerSurfaceEnvelope;
    }
    if ('timers' in published) fitNativePresentation(published, ordered.length, copy.remaining);
    const [current, ...duplicates] = factory().getInstances();
    for (const duplicate of duplicates) await duplicate.end('immediate');
    if (current) await current.update(published);
    const instance = current ?? factory().start(published, 'hourpaths:///home');
    const generation = ++tokenGeneration;
    tokenSubscription?.remove();
    tokenSubscription = undefined;
    if (refresh && props.owner && props.accountLease && props.activityExpiresAt) {
      const registration = {
        owner: props.owner, activityId: props.accountLease, accountLease: props.accountLease,
        expiresAt: props.activityExpiresAt,
        boundaries: props.timers.flatMap(timer => timer.goalPeriods?.map(period => period.endsAt) ?? []),
      };
      const register = (token: string | null) => {
        if (token && generation === tokenGeneration) void refresh.update({ ...registration, token });
      };
      let receivedRotation = false;
      tokenSubscription = instance.addPushTokenListener(event => { receivedRotation = true; register(event.pushToken); });
      void instance.getPushToken().then(token => { if (!receivedRotation) register(token); }).catch(() => undefined);
    }
  },
  });
};
