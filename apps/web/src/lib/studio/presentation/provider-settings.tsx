import { useSearch } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { IdentityProvider, LinkedProvider, ProviderSettingsService } from '@hourpaths/client-core';

export function ProviderSettings({ service, i18n }: { service: ProviderSettingsService; i18n: Translator }) {
  const callback = useSearch({ strict: false }) as { identityLink?: string };
  const admission = useRef(false);
  const [providers, setProviders] = useState<LinkedProvider[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [review, setReview] = useState<{ provider: IdentityProvider; owner: string } | null>(null);
  const [attempt, retry] = useState(0);
  useEffect(() => {
    let active = true;
    setProviders(null); setFailed(false);
    void service.list().then(value => { if (active) setProviders(value); }).catch(() => { if (active) setFailed(true); });
    return () => { active = false; };
  }, [service, attempt]);
  async function change(work: () => Promise<void>) {
    if (admission.current) return;
    admission.current = true;
    setBusy(true); setFailed(false);
    try { await work(); setReview(null); setProviders(await service.list()); }
    catch { setFailed(true); }
    finally { admission.current = false; setBusy(false); }
  }
  const name = (provider: IdentityProvider) => i18n.t(provider === 'google' ? 'identity.google' : 'identity.apple');
  return <section className="studio-settings-card">
    <h3>{i18n.t('identity.heading')}</h3>{callback.identityLink === 'success' && <p role="status">{i18n.t('identity.success')}</p>}{callback.identityLink === 'failed' && <p role="alert">{i18n.t('identity.failed')}</p>}<p>{i18n.t('identity.description')}</p>
    {!providers && !failed && <p role="status">{i18n.t('common.loading')}</p>}
    {providers && <ul className="studio-settings-people">{(['google', 'apple'] as const).map(provider => {
      const linked = providers.find(item => item.provider === provider);
      return <li key={provider}><div><strong>{name(provider)}</strong><small>{i18n.t(linked ? 'identity.connected' : 'identity.notConnected')}</small></div>
        {linked ? <button disabled={busy || !linked.canUnlink} onClick={() => { const owner = service.owner(); if (owner) setReview({ provider, owner }); }}>{i18n.t('identity.unlink')}</button>
          : <button disabled={busy} onClick={() => void change(() => service.link(provider))}>{i18n.t('identity.link')}</button>}
      </li>;
    })}</ul>}
    {providers?.length === 1 && <p>{i18n.t('identity.keepOne')}</p>}
    {review && <div className="studio-settings-review"><p>{i18n.t('identity.unlinkConfirm', { provider: name(review.provider) })}</p>
      <button disabled={busy} onClick={() => setReview(null)}>{i18n.t('common.cancel')}</button>
      <button disabled={busy} onClick={() => void change(() => service.unlink(review.provider, review.owner))}>{i18n.t('identity.unlink')}</button></div>}
    {busy && <p role="status">{i18n.t('identity.working')}</p>}
    {failed && <div role="alert"><p>{i18n.t('identity.failed')}</p><button disabled={busy} onClick={() => retry(value => value + 1)}>{i18n.t('common.retry')}</button></div>}
  </section>;
}
