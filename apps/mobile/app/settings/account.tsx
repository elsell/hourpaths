import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Alert } from 'react-native';
import type { MessageKey } from '@hourpaths/i18n';
import { createDeviceTranslator } from '../../src/i18n';
import { queueSettingsJourneyBootstrap, scheduleSettingsJourneyBootstrap } from '../../src/settings-journey-route-recovery';
import { ownsSettingsRouteState, settingsRouteOwnerDecision } from '../../src/settings-route-state';
import { useSettingsJourneyRouteAncestry } from '../../src/use-settings-journey-route-ancestry';
import { ThemedText as Text } from '../../src/ui/primitives';
import { SettingsJourneyRecoveryView } from '../../src/ui/settings-journey-recovery-view';
import { useSettingsJourneyRecovery } from '../../src/ui/settings-journey-route-presentation';
import { SettingsActionRow, SettingsSection, SettingsSeparator, SettingsShell, SettingsValueRow } from '../../src/ui/settings-list';
import { useSettingsPresentation, type SignOutTimerChoice } from '../../src/ui/settings-presentation';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'account', routeKey: 'settings:account' } as const;

export default function AccountSettings() {
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
  const [failureKey, setFailureKey] = useState<MessageKey | null>(null);
  const [working, setWorking] = useState(false);
  const [stateOwnerKey, setStateOwnerKey] = useState<string | null>(null);
  const activeOwnerKey = useRef<string | null>(presentation?.sessionKey ?? null);
  const admission = useRef(false);
  activeOwnerKey.current = presentation?.sessionKey ?? null;
  useEffect(() => {
    if (!presentation) return;
    setStateOwnerKey(presentation.sessionKey);
    setFailureKey(null);
    setWorking(false);
    admission.current = false;
  }, [presentation?.sessionKey]);
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
  const activePresentation = presentation;
  const ownsState = ownsSettingsRouteState(stateOwnerKey, activePresentation.sessionKey);
  const ownedWorking = ownsState && working;
  const ownedFailureKey = ownsState ? failureKey : null;
  const name = activePresentation.displayName || activePresentation.email;

  async function signOut(resolution: SignOutTimerChoice) {
    if (admission.current) return;
    const ownedSessionKey = activePresentation.sessionKey;
    admission.current = true;
    setFailureKey(null);
    setWorking(true);
    try {
      const result = await activePresentation.signOut(resolution);
      if (activeOwnerKey.current !== ownedSessionKey || !activePresentation.isCurrent()) return;
      if (result.kind === 'signed_out') router.dismissAll();
      else if (result.kind === 'failed') setFailureKey('settings.account.activeTimers.stopFailed');
      else if (result.kind === 'choice_required') confirmActiveTimerSignOut(result.runningTimerCount);
    } finally {
      if (activeOwnerKey.current === ownedSessionKey) {
        admission.current = false;
        setWorking(false);
      }
    }
  }

  function confirmActiveTimerSignOut(runningTimerCount: number) {
    Alert.alert(
        i18n.t('settings.account.activeTimers.title'),
        i18n.t('settings.account.activeTimers.message', { count: runningTimerCount }),
        [
          { style: 'cancel', text: i18n.t('common.cancel') },
          {
            onPress: () => void signOut('keep_running'),
            text: i18n.t('settings.account.activeTimers.keepRunningAndSignOut'),
          },
          {
            onPress: () => void signOut('stop_and_save'),
            style: 'destructive',
            text: i18n.t('settings.account.activeTimers.stopAndSignOut'),
          },
        ],
    );
  }

  function confirmSignOut() {
    if (activePresentation.runningTimerCount > 0) {
      confirmActiveTimerSignOut(activePresentation.runningTimerCount);
      return;
    }
    Alert.alert(
      i18n.t('settings.account.signOutConfirmTitle'),
      i18n.t('settings.account.signOutConfirmMessage', { name }),
      [
        { style: 'cancel', text: i18n.t('common.cancel') },
        { onPress: () => void signOut('confirmed_no_timers'), style: 'destructive', text: i18n.t('auth.signOut') },
      ],
    );
  }

  return <SettingsShell>
    <SettingsSection footer={i18n.t('settings.account.footer')}>
      <SettingsValueRow label={i18n.t('settings.account.signedInAs')} value={name} />
      <SettingsSeparator />
      <SettingsValueRow label={i18n.t('settings.account.email')} value={activePresentation.email} />
    </SettingsSection>
    <SettingsSection footer={ownedFailureKey ? i18n.t(ownedFailureKey) : undefined}>
      <SettingsActionRow
        accessibilityLabel={i18n.t('settings.account.signOutLabel', { name })}
        disabled={ownedWorking}
        label={i18n.t('auth.signOut')}
        onPress={confirmSignOut}
      />
      {ownedWorking ? <Text accessibilityLiveRegion="polite" accessibilityRole="progressbar">
        {i18n.t('settings.account.signingOut')}
      </Text> : null}
    </SettingsSection>
  </SettingsShell>;
}
