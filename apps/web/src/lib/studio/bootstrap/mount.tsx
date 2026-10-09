import { apiEnforcement, apiReporting } from '@hourpaths/client-core';
import { AccountExportController, apiAccountExport } from '@hourpaths/client-core';
import { browserAccountExportJSON } from '../account/adapters/browser-account-export';
import { apiPolicyTimers, PolicyTimersController, retainedPolicyTimers } from '@hourpaths/client-core';
import { openWebPolicyLink } from '../../external-policy-link';
import { apiPolicyRenewal, PolicyReviewController } from '@hourpaths/client-core';
import { readBrowserPictureFile } from '../preferences/adapters/browser-picture-file';
import { browserProviderIdentities } from '../account/adapters/browser-provider-identities';
import { browserAccountDeletion } from '../account/adapters/browser-account-deletion';
import type { AccountDeletionService } from '../account/ports/account-deletion';
import { AccountDeletionRecovery } from '../presentation/account-deletion';
import { browserSessionState } from '../../browser-session-state';
import { durableActivityRepository } from '../offline/adapters/durable-activity-repository';
import { ActivityFailure } from '../history/domain/detail';
import { retainedTimers } from '../offline/adapters/retained-timers';
import { RetainedTimerControls } from '../presentation/retained-timers';
import { browserConnected, subscribeTrackingConnectivity } from '../offline/adapters/browser-tracking-connectivity';
import { browserTrackingRuntime } from '../offline/adapters/browser-tracking-runtime';
import { IndexedDBTrackingStore } from '../offline/adapters/indexeddb-tracking-store';
import { durablePathRepository } from '../offline/adapters/durable-path-repository';
import { PathRequestError } from '../paths/adapters/path-mapping';
import { TrackingReplayWorker, type ClientRuntimeConfig } from '@hourpaths/client-core';
import { browserEntryService } from '../entry/adapters/browser-entry-service';
import { AccountEntry } from '../presentation/account-entry';
import { apiNudgesRepository } from '../nudges/adapters/api-nudges-repository';
import { apiBlockingRepository } from '../blocking/adapters/api-blocking-repository';
import { apiNotifications } from '../notifications/adapters/api-notifications';
import { apiOwnershipRepository } from '../ownership/adapters/api-ownership-repository';
import { apiSharingRepository } from '../sharing/adapters/api-sharing-repository';
import { accountSession } from '../session/application/account-session';
import { apiPreferencesRepository } from '../preferences/adapters/api-preferences-repository';
import { apiStatisticsRepository } from '../analytics/adapters/api-statistics-repository';
import { apiSocialRepository } from '../social/adapters/api-social-repository';
import { createRoot } from 'react-dom/client';
import { createTranslator, type SupportedLocale } from '@hourpaths/i18n';
import { apiPathRepository } from '../paths/adapters/api-path-repository';
import { browserSessionStore, apiSessionService } from '../session/adapters/browser-session';
import { SessionController } from '../session/application/session-controller';
import { historyRepository } from '../history/application/history';
import { durableHistoryRepository } from '../offline/adapters/durable-history-repository';
import { HistoryRequestError } from '../history/adapters/api-history-source';
import { apiHistorySource } from '../history/adapters/api-history-source';
import { apiActivityRepository } from '../history/adapters/api-activity-repository';
import { StudioApp } from '../presentation/app';

export function mountStudio(element: HTMLElement, options: { apiURL: string; locale: SupportedLocale; config: ClientRuntimeConfig }) {
  let cancelled = false;
  let generation = 0;
  let recovering = false;
  let dispose: (() => void) | undefined;
  let unsubscribe: (() => void) | undefined;
  let previous: string | null | undefined;
  const sessions = browserSessionStore();
  const local = new IndexedDBTrackingStore();
  const i18n = createTranslator([options.locale]);
  const recovery = (owners: string[]) => {
    recovering = true;
    dispose?.();
    const root = createRoot(element);
    root.render(<AccountDeletionRecovery service={deletion} owners={owners} i18n={i18n} signIn={() => {
      return browserEntryService(options.config).begin();
    }} />);
    dispose = () => root.unmount();
  };
  const deletion = browserAccountDeletion(options.apiURL, sessions, local, owner => {
    if (!cancelled && !recovering && sessions.read()?.ownerId === owner) { generation++; recovery([owner]); }
  }, () => { if (!cancelled) { previous = undefined; render(); } });
  const render = () => {
    if (cancelled) return;
    let current: string | null;
    try { current = browserSessionState().read(); } catch { current = null; }
    if (dispose && previous === current) return;
    previous = current;
    recovering = false;
    const own = ++generation;
    dispose?.(); dispose = undefined;
    // No API reads or offline replay are started before durable intent discovery.
    void deletion.pendingOwners().then(owners => {
      if (cancelled || own !== generation) return;
      if (owners.length) recovery(owners);
      else dispose = mountReadyStudio(element, options, deletion);
    }).catch(() => {
      if (cancelled || own !== generation) return;
      const root = createRoot(element);
      root.render(<main className="studio studio-entry"><section className="studio-entry-card"><p role="alert">{i18n.t('errors.localSessionUnreadable')}</p><button onClick={() => { previous = undefined; render(); }}>{i18n.t('common.retry')}</button></section></main>);
      dispose = () => root.unmount();
    });
  };
  void sessions.initialize!().catch(() => undefined).then(() => {
    if (cancelled) return;
    render();
    try { unsubscribe = browserSessionState().subscribe(render); }
    catch { /* Account entry presents unavailable browser storage. */ }
  });
  return () => { cancelled = true; generation++; unsubscribe?.(); dispose?.(); void local.close(); };
}

function mountReadyStudio(element: HTMLElement, options: { apiURL: string; locale: SupportedLocale; config: ClientRuntimeConfig }, deletion: AccountDeletionService) {
  const root = createRoot(element);
  const i18n = createTranslator([options.locale]);
  const store = browserSessionStore();
  const initial = store.read();
  let active = true;
  let stopRetained: (() => void) | undefined;
  let stopHistoryRefresh: (() => void) | undefined;
  let offline: ReturnType<typeof browserTrackingRuntime> | undefined;
  const unavailable = () => {
    stopHistoryRefresh?.();
    offline?.dispose();
    stopRetained?.(); stopRetained = undefined;
    if (!active) return;
    const owner = store.retainedOwner?.();
    if (owner) {
      const local = new IndexedDBTrackingStore();
      const retained = retainedTimers(owner, local, () => active && store.retainedOwner?.() === owner, () => Date.now(), () => crypto.randomUUID());
      stopRetained = () => { retained.dispose(); void local.close(); };
      root.render(<AccountEntry service={browserEntryService(options.config)} i18n={i18n} retained={<RetainedTimerControls service={retained} i18n={i18n} />} />);
    } else root.render(<AccountEntry service={browserEntryService(options.config)} i18n={i18n} />);
  };
  if (!initial || initial.expiresAt <= Date.now() || initial.destination !== 'home') {
    unavailable();
    return () => { active = false; stopRetained?.(); root.unmount(); };
  }
  const session = new SessionController(initial, store, apiSessionService(options.apiURL), () => Date.now(), unavailable);
  // Validate the local owner/expiry even while the user is idle.
  const deadline = setInterval(() => { void session.maintain(); }, 1000);
  const durableStore = new IndexedDBTrackingStore();
  const policyTimers = new PolicyTimersController(
    apiPolicyTimers(options.apiURL, () => session.token(), credential => session.reject(credential)),
    () => crypto.randomUUID(),
    retainedPolicyTimers(async () => durableStore, () => policyReview.state.review?.userId ?? null, () => active && !!session.token(), () => Date.now(), () => crypto.randomUUID()),
  );
  const accountExport = new AccountExportController({
    owner: () => policyReview.state.review?.userId ?? null, current: () => active && !!session.token(),
    repository: apiAccountExport(options.apiURL, () => session.token(), credential => session.reject(credential)),
    now: () => Date.now(), device: owner => durableStore.read(owner), sink: browserAccountExportJSON,
  });
  const policyReview: PolicyReviewController = new PolicyReviewController(apiPolicyRenewal(options.apiURL, () => session.token(), initial.ownerId, credential => session.reject(credential)), () => crypto.randomUUID(), url => openWebPolicyLink(url, () => { throw new Error("policy_link_unavailable"); }), policyTimers, accountExport);
  offline = browserTrackingRuntime(options.apiURL, session, durableStore, browserConnected);
  const remotePaths = apiPathRepository(options.apiURL, () => session.token(), credential => session.reject(credential), false);
  const paths = durablePathRepository(
    remotePaths,
    durableStore, offline.runtime,
    error => error instanceof TypeError || error instanceof PathRequestError && (error.status === 0 || error.status >= 500 || error.status === 429),
  );
  const history = durableHistoryRepository(
    historyRepository(apiHistorySource(options.apiURL, () => session.token(), remotePaths, credential => session.reject(credential))),
    offline.runtime,
    error => error instanceof TypeError || error instanceof HistoryRequestError && (error.status === 429 || error.status >= 500),
    25, () => Date.now(),
  );
  const historyRefresh = new TrackingReplayWorker(async () => {
    if (!await history.refresh()) throw new Error('history_refresh_unavailable');
  }, (work, delay) => { const timer = setTimeout(work, delay); return () => clearTimeout(timer); }, () => {});
  stopHistoryRefresh = () => historyRefresh.dispose();
  const wakeTracking = () => {
    if (browserConnected()) void policyReview.refresh();
    offline?.retry();
    if (browserConnected()) void historyRefresh.wake();
  };
  if (browserConnected()) void historyRefresh.wake();
  const stopConnectivity = subscribeTrackingConnectivity(wakeTracking);
  root.render(<StudioApp dependencies={{
    policyReview,
    providers: browserProviderIdentities(options.config),
    deletion,
    offline,
    paths,
    enforcement: {
      list: cursor => apiEnforcement(options.apiURL, session.token(), credential => session.reject(credential)).list(cursor),
      get: id => apiEnforcement(options.apiURL, session.token(), credential => session.reject(credential)).get(id),
      appeal: (id, explanation, key) => apiEnforcement(options.apiURL, session.token(), credential => session.reject(credential)).appeal(id, explanation, key),
    },
    reporting: { submit: (draft, key, signal) => apiReporting(options.apiURL, session.token(), credential => session.reject(credential), signal).submit(draft, key) },
    blocking: apiBlockingRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    notifications: apiNotifications(options.apiURL, () => session.token(), credential => session.reject(credential), i18n),
    ownership: apiOwnershipRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    nudges: apiNudgesRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    sharing: apiSharingRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    activities: durableActivityRepository(apiActivityRepository(options.apiURL, () => session.token(), credential => session.reject(credential)), offline.runtime, error => error instanceof TypeError || error instanceof ActivityFailure && error.retryable),
    session: accountSession(paths, () => !!session.token(), () => session.signOut(), () => crypto.randomUUID()),
    pictureFiles:{read:readBrowserPictureFile},
    preferences: apiPreferencesRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    statistics: apiStatisticsRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    social: apiSocialRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    history,
    accountScope: crypto.randomUUID(),
    i18n,
    operationId: () => crypto.randomUUID(),
    now: () => Date.now(),
  }} />);
  return () => { active = false; policyReview.dispose(); clearInterval(deadline); stopConnectivity(); historyRefresh.dispose(); offline?.dispose(); stopRetained?.(); session.dispose(); root.unmount(); };
}

export function mountAccountEntry(element: HTMLElement, options: { config: ClientRuntimeConfig; locale: SupportedLocale; callback?: boolean }) {
  const root = createRoot(element);
  root.render(<AccountEntry service={browserEntryService(options.config)} i18n={createTranslator([options.locale])} callback={options.callback} />);
  return () => root.unmount();
}
