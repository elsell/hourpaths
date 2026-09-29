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
  const deadline = setInterval(() => { session.token(); }, 1000);
  const paths = apiPathRepository(options.apiURL, () => session.token(), () => session.reject());
  root.render(<StudioApp dependencies={{
    paths,
    social: apiSocialRepository(options.apiURL, () => session.token(), () => session.reject()),
    history: historyRepository(apiHistorySource(options.apiURL, () => session.token(), paths, () => session.reject())),
    accountScope: crypto.randomUUID(),
    i18n,
    operationId: () => crypto.randomUUID(),
    now: () => Date.now(),
  }} />);
  return () => { active = false; clearInterval(deadline); session.dispose(); root.unmount(); };
}
