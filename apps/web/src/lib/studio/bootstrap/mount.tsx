import type { ClientRuntimeConfig } from '@hourpaths/client-core';
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
import { apiHistorySource } from '../history/adapters/api-history-source';
import { apiActivityRepository } from '../history/adapters/api-activity-repository';
import { StudioApp } from '../presentation/app';

export function mountStudio(element: HTMLElement, options: { apiURL: string; locale: SupportedLocale; config: ClientRuntimeConfig }) {
  const root = createRoot(element);
  const i18n = createTranslator([options.locale]);
  const store = browserSessionStore();
  const initial = store.read();
  let active = true;
  const unavailable = () => {
    if (active) root.render(<AccountEntry service={browserEntryService(options.config)} i18n={i18n} />);
  };
  if (!initial || initial.expiresAt <= Date.now() || initial.destination !== 'home') {
    unavailable();
    return () => { active = false; root.unmount(); };
  }
  const session = new SessionController(initial, store, apiSessionService(options.apiURL), () => Date.now(), unavailable);
  // Validate the local owner/expiry even while the user is idle.
  const deadline = setInterval(() => { void session.maintain(); }, 1000);
  const paths = apiPathRepository(options.apiURL, () => session.token(), credential => session.reject(credential));
  root.render(<StudioApp dependencies={{
    paths,
    blocking: apiBlockingRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    notifications: apiNotifications(options.apiURL, () => session.token(), credential => session.reject(credential), i18n),
    ownership: apiOwnershipRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    nudges: apiNudgesRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    sharing: apiSharingRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    activities: apiActivityRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    session: accountSession(paths, () => !!session.token(), () => session.signOut(), () => crypto.randomUUID()),
    preferences: apiPreferencesRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    statistics: apiStatisticsRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    social: apiSocialRepository(options.apiURL, () => session.token(), credential => session.reject(credential)),
    history: historyRepository(apiHistorySource(options.apiURL, () => session.token(), paths, credential => session.reject(credential))),
    accountScope: crypto.randomUUID(),
    i18n,
    operationId: () => crypto.randomUUID(),
    now: () => Date.now(),
  }} />);
  return () => { active = false; clearInterval(deadline); session.dispose(); root.unmount(); };
}

export function mountAccountEntry(element: HTMLElement, options: { config: ClientRuntimeConfig; locale: SupportedLocale; callback?: boolean }) {
  const root = createRoot(element);
  root.render(<AccountEntry service={browserEntryService(options.config)} i18n={createTranslator([options.locale])} callback={options.callback} />);
  return () => root.unmount();
}
