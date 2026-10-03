import { useRef, useState } from 'react';
import type { Translator } from '@hourpaths/i18n';
import { ThemedText } from './primitives';
import { SettingsActionRow, SettingsSection, SettingsShell } from './settings-list';

/** Presentation only: confirmation is bound to the account reviewed by its caller. */
export function AccountDeletionView({ i18n, name, confirm, cancel, signIn, recovery = false }: {
  i18n: Translator;
  name?: string;
  confirm(): Promise<void>;
  cancel?: () => void;
  signIn?: () => Promise<void>;
  recovery?: boolean;
}) {
  const admitted = useRef(false);
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false), [confirmed, setConfirmed] = useState(recovery);
  async function submit() {
    if (admitted.current) return;
    admitted.current = true; setConfirmed(true); setBusy(true); setFailed(false);
    try { await confirm(); }
    catch { setFailed(true); }
    finally { admitted.current = false; setBusy(false); }
  }
  return <SettingsShell>
    <SettingsSection title={i18n.t(recovery ? 'accountDelete.recovery' : 'accountDelete.heading')}>
      {name ? <ThemedText>{name}</ThemedText> : null}
      {recovery ? <ThemedText>{i18n.t('accountDelete.recoveryDetail')}</ThemedText> : (['warning', 'paths', 'timers', 'retention'] as const).map(key => <ThemedText key={key}>{i18n.t(`accountDelete.${key}`)}</ThemedText>)}
    </SettingsSection>
    <SettingsSection>
      {failed ? <ThemedText accessibilityRole="alert">{i18n.t('accountDelete.failed')}</ThemedText> : null}
      {busy ? <ThemedText accessibilityLiveRegion="polite">{i18n.t('accountDelete.progress')}</ThemedText> : null}
      {!confirmed && cancel ? <SettingsActionRow tone="default" accessibilityLabel={i18n.t('common.cancel')} label={i18n.t('common.cancel')} onPress={cancel} /> : null}
      {failed && signIn ? <SettingsActionRow disabled={busy} tone="default" accessibilityLabel={i18n.t('auth.signIn')} label={i18n.t('auth.signIn')} onPress={() => { void signIn().catch(() => setFailed(true)); }} /> : null}
      <SettingsActionRow disabled={busy} accessibilityLabel={i18n.t(confirmed ? 'common.retry' : 'accountDelete.confirm')} label={i18n.t(confirmed ? 'common.retry' : 'accountDelete.confirm')} onPress={() => void submit()} />
    </SettingsSection>
  </SettingsShell>;
}
