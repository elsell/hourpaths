import { Link } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import type { Translator } from '@hourpaths/i18n';
export function StudioShell({ children, page, i18n }: { children: ReactNode; page: 'paths' | 'following' | 'stats' | 'settings' | 'invitations'; i18n: Translator }) {
  return <div className={`studio ${page === 'following' || page === 'settings' ? 'studio-social-shell' : ''}`}>
    <aside className="studio-nav">
      <Link className="studio-brand" to="/"><span aria-hidden="true">◉</span> HourPaths</Link>
      <nav aria-label={i18n.t('studio.navigation')}>
        <Link to="/" aria-current={page === 'paths' ? 'page' : undefined}>{i18n.t('studio.paths')}</Link>
        <Link to="/following" aria-current={page === 'following' ? 'page' : undefined}>{i18n.t('studio.social.following')}</Link>
        <Link to="/stats" aria-current={page === 'stats' ? 'page' : undefined}>{i18n.t('stats.title')}</Link>
        <Link to="/settings/account" aria-current={page === 'settings' ? 'page' : undefined}>{i18n.t('settings.heading')}</Link>
        <Link to="/invitations" aria-current={page === 'invitations' ? 'page' : undefined}>{i18n.t('pathInvitation.pendingHeading')}</Link>
      </nav>
      <a className="studio-legacy" href="/">{i18n.t('studio.currentApp')}</a>
    </aside>{children}
  </div>;
}
