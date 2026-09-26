import { router } from 'expo-router';
import { getLocales } from 'expo-localization';
import { useEffect, useRef } from 'react';
import { createDeviceTranslator } from '../../src/i18n';
import { queueSettingsJourneyBootstrap, scheduleSettingsJourneyBootstrap } from '../../src/settings-journey-route-recovery';
import { settingsRouteOwnerDecision } from '../../src/settings-route-state';
import { useSettingsJourneyRouteAncestry } from '../../src/use-settings-journey-route-ancestry';
import { SettingsIcon } from '../../src/ui/settings-icon';
import { SettingsJourneyRecoveryView } from '../../src/ui/settings-journey-recovery-view';
import { useSettingsJourneyRecovery } from '../../src/ui/settings-journey-route-presentation';
import { SettingsNavigationRow, SettingsSection, SettingsSeparator, SettingsShell } from '../../src/ui/settings-list';
import { useSettingsPresentation } from '../../src/ui/settings-presentation';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'settings', routeKey: 'settings:root' } as const;

export default function Settings() {
  const publishedPresentation = useSettingsPresentation();
  const publishedRecovery = useSettingsJourneyRecovery(intent.routeKey);
  const lastRecoverySessionKey = useRef<string | undefined>(undefined);
  const incomingSessionKey = publishedPresentation?.sessionKey ?? publishedRecovery?.sessionKey;
  const ownerDecision = settingsRouteOwnerDecision(lastRecoverySessionKey.current, incomingSessionKey);
  if (ownerDecision === 'claim') lastRecoverySessionKey.current = incomingSessionKey;
  const ownerReplaced = ownerDecision === 'replacement';
  const presentation = ownerReplaced ? undefined : publishedPresentation;
  const recovery = ownerReplaced ? undefined : publishedRecovery;
  useSettingsJourneyRouteAncestry(intent);
  useEffect(() => {
    if (ownerReplaced) router.replace('/(tabs)/home');
  }, [ownerReplaced]);
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleSettingsJourneyBootstrap(intent, () => router.replace('/(tabs)/home'), lastRecoverySessionKey.current);
  }, [presentation, recovery]);
  if (!presentation) return <SettingsJourneyRecoveryView
    i18n={i18n}
    intent={intent}
    onHome={() => router.replace('/(tabs)/home')}
    onRetry={recovery?.retry ?? (() => {
      queueSettingsJourneyBootstrap(intent, lastRecoverySessionKey.current);
      router.replace('/(tabs)/home');
    })}
    state={recovery?.state ?? 'loading'}
  />;
  const name = presentation.displayName || presentation.email;
  return <SettingsShell>
    <SettingsSection title={i18n.t('settings.account')}>
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('settings.account.openLabel', { email: presentation.email, name })}
        context={presentation.email}
        icon={<SettingsIcon systemName="person.crop.circle" />}
        label={name}
        onPress={() => router.push('/settings/account')}
      />
    </SettingsSection>
    <SettingsSection>
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('notification.settings.openLabel')}
        icon={<SettingsIcon systemName="bell" />}
        label={i18n.t('notification.settings.heading')}
        onPress={() => router.push('/settings/notifications')}
      />
      <SettingsSeparator />
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('settings.timeZone.openLabel')}
        icon={<SettingsIcon systemName="globe" />}
        label={i18n.t('settings.timeZone.heading')}
        onPress={() => router.push('/settings/time-zone')}
      />
      <SettingsSeparator />
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('settings.interactions.openLabel')}
        icon={<SettingsIcon systemName="person.2" />}
        label={i18n.t('settings.interactions.heading')}
        onPress={() => router.push('/settings/interactions')}
      />
      <SettingsSeparator />
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('blocking.settingsOpenLabel')}
        icon={<SettingsIcon systemName="hand.raised" />}
        label={i18n.t('blocking.settingsHeading')}
        onPress={() => router.push('/settings/blocked-accounts')}
      />
    </SettingsSection>
  </SettingsShell>;
}
