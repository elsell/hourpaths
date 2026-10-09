import { useEffect, useState } from 'react';
import type { AccountExportController } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { ActionButton, ThemedText } from './primitives';
import { SettingsSection } from './settings-list';

export function AccountExport({ controller, i18n }: { controller: AccountExportController; i18n: Translator }) {
  const [state, setState] = useState(controller.state);
  useEffect(() => { setState(controller.state); return controller.subscribe(() => setState(controller.state)); }, [controller]);
  const busy = state.phase === 'collecting' || state.phase === 'saving';
  return <SettingsSection>
    <ActionButton variant="secondary" busy={busy} disabled={busy} label={i18n.t('accountExport.heading')} onPress={() => void controller.save()} />
    {busy && <ThemedText accessibilityLiveRegion="polite">{i18n.t(state.phase === 'collecting' ? 'accountExport.preparing' : 'accountExport.saving')}</ThemedText>}
    {state.phase === 'collecting' && <ActionButton variant="quiet" label={i18n.t('common.cancel')} onPress={() => controller.cancel()} />}
    {state.phase === 'failed' && <ThemedText accessibilityRole="alert">{i18n.t('accountExport.failed')}</ThemedText>}
    {state.phase === 'saved' && <ThemedText accessibilityLiveRegion="polite">{i18n.t('accountExport.ready')}</ThemedText>}
  </SettingsSection>;
}
