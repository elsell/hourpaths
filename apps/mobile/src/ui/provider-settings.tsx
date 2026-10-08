import { useEffect, useRef, useState } from 'react';
import type { IdentityProvider, LinkedProvider, ProviderSettingsService } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { SettingsActionRow, SettingsNavigationRow, SettingsSection, SettingsSeparator, SettingsValueRow } from './settings-list';

export function ProviderSettings({ service, i18n, confirmUnlink }: { service: ProviderSettingsService; i18n: Translator; confirmUnlink(provider: IdentityProvider, confirmed: () => void): void }) {
  const [providers, setProviders] = useState<LinkedProvider[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const [attempt, retry] = useState(0);
  const alive = useRef(true);
  const admission = useRef(false);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => {
    let active = true;
    setFailed(false);
    void service.list().then(rows => { if (active) setProviders(rows); }).catch(() => { if (active) setFailed(true); });
    return () => { active = false; };
  }, [service, attempt]);
  const name = (provider: IdentityProvider) => i18n.t(provider === 'google' ? 'identity.google' : 'identity.apple');
  async function change(work: () => Promise<void>) {
    if (admission.current) return;
    admission.current = true; setBusy(true); setFailed(false);
    try { await work(); const rows = await service.list(); if (alive.current) setProviders(rows); }
    catch { if (alive.current) setFailed(true); }
    finally { admission.current = false; if (alive.current) setBusy(false); }
  }
  function select(provider: IdentityProvider) {
    if (busy) return;
    const linked = providers?.find(row => row.provider === provider);
    if (!linked) { void change(() => service.link(provider)); return; }
    const owner = service.owner();
    if (!linked.canUnlink || !owner) return;
    confirmUnlink(provider, () => { if (alive.current) void change(() => service.unlink(provider, owner)); });
  }
  return <SettingsSection title={i18n.t('identity.heading')} footer={failed ? i18n.t('identity.failed') : providers?.length === 1 ? i18n.t('identity.keepOne') : i18n.t('identity.description')}>
    {!providers && !failed ? <SettingsValueRow label={i18n.t('common.loading')} value="" /> : null}
    {providers ? (['google', 'apple'] as const).map((provider, index) => {
      const linked = providers.find(row => row.provider === provider);
      return <ProviderRow key={provider} separator={index > 0} label={name(provider)} status={i18n.t(linked ? 'identity.connected' : 'identity.notConnected')} actionLabel={i18n.t(linked ? 'identity.unlink' : 'identity.link')} accessibilityLabel={i18n.t(linked ? 'identity.unlinkProvider' : 'identity.linkProvider', { provider: name(provider) })} disabled={busy || Boolean(linked && !linked.canUnlink)} press={() => select(provider)} />;
    }) : null}
    {busy ? <SettingsValueRow label={i18n.t('identity.working')} value="" /> : null}
    {failed ? <SettingsActionRow tone="default" disabled={busy} label={i18n.t('common.retry')} accessibilityLabel={i18n.t('common.retry')} onPress={() => retry(value => value + 1)} /> : null}
  </SettingsSection>;
}
function ProviderRow({ separator, label, status, actionLabel, accessibilityLabel, disabled, press }: { separator: boolean; label: string; status: string; actionLabel: string; accessibilityLabel: string; disabled: boolean; press(): void }) {
  return <>{separator && <SettingsSeparator />}<SettingsNavigationRow label={label} context={status} value={actionLabel} accessibilityLabel={accessibilityLabel} disabled={disabled} onPress={press} /></>;
}
