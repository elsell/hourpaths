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
import { StudioApp } from '../presentation/app';

export function mountStudio(element: HTMLElement, options: { apiURL: string; locale: SupportedLocale }) {
  const root = createRoot(element);
  const i18n = createTranslator([options.locale]);
  const store = browserSessionStore();
  const initial = store.read();
  let active = true;
  const unavailable = () => {
    if (active) root.render(<a href="/">{i18n.t('auth.signIn')}</a>);
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
