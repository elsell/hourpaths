import * as Crypto from 'expo-crypto';
import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import {
  createNudgeChannelOperationOwner,
  reviewNudgeChannelChange,
  type NudgeChannelPreference,
} from '@hourpaths/client-core';
import {
  getNativePushPermission,
  openNativePushSettings,
  subscribeNativeAppActive,
} from '../../src/push-notifications-native';
import type { PushPermission } from '../../src/push-notifications';
import { ownsNotificationSettingsState } from '../../src/notification-settings-state';
import { createDeviceTranslator } from '../../src/i18n';
import {
  queueNotificationJourneyBootstrap,
  scheduleNotificationJourneyBootstrap,
} from '../../src/notification-journey-route-recovery';
import { NotificationJourneyRecoveryView } from '../../src/ui/notification-journey-recovery-view';
import { useNotificationJourneyRecovery } from '../../src/ui/notification-route-presentation';
import {
  SettingsActionRow,
  SettingsSection,
  SettingsSeparator,
  SettingsSwitchRow,
  SettingsShell,
  SettingsValueRow,
} from '../../src/ui/settings-list';
import { useSettingsPresentation } from '../../src/ui/settings-presentation';
import { useNotificationJourneyRouteAncestry } from '../../src/use-notification-journey-route-ancestry';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'notification-settings', routeKey: 'notifications:settings' } as const;

export default function NotificationSettings() {
  const presentation = useSettingsPresentation();
  const recovery = useNotificationJourneyRecovery(intent.routeKey);
  const lastRecoverySessionKey = useRef<string | undefined>(undefined);
  if (recovery?.sessionKey) lastRecoverySessionKey.current = recovery.sessionKey;
  useNotificationJourneyRouteAncestry(intent);
  const [permission, setPermission] = useState<PushPermission | null>(null);
  const [stateOwnerKey, setStateOwnerKey] = useState<string | null>(null);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState(false);
  const [nudgeChannelPreference, setNudgeChannelPreference] = useState<NudgeChannelPreference | null>(null);
  const [nudgeChannelLoading, setNudgeChannelLoading] = useState(true);
  const [nudgeChannelSaving, setNudgeChannelSaving] = useState(false);
  const [nudgeChannelError, setNudgeChannelError] = useState(false);
  const [nudgeChannelOwner] = useState(() => createNudgeChannelOperationOwner(() => Crypto.randomUUID()));
  const nudgeChannelLoadRevision = useRef(0);
  const activeOwnerKey = useRef<string | null>(null);
  activeOwnerKey.current = presentation?.sessionKey ?? null;

  async function loadNudgeChannel() {
    if (!presentation) return;
    const ownedSessionKey = presentation.sessionKey;
    const revision = ++nudgeChannelLoadRevision.current;
    setNudgeChannelLoading(true);
    setNudgeChannelError(false);
    try {
      const preference = await presentation.getNudgeChannelPreference();
      if (revision === nudgeChannelLoadRevision.current && activeOwnerKey.current === ownedSessionKey) {
        setNudgeChannelPreference(preference);
      }
    } catch {
      if (revision === nudgeChannelLoadRevision.current && activeOwnerKey.current === ownedSessionKey) {
        setNudgeChannelError(true);
      }
    } finally {
      if (revision === nudgeChannelLoadRevision.current && activeOwnerKey.current === ownedSessionKey) {
        setNudgeChannelLoading(false);
      }
    }
  }

  useEffect(() => {
    if (!presentation) return;
    const ownedSessionKey = presentation.sessionKey;
    setStateOwnerKey(ownedSessionKey);
    setPermission(null);
    setWorking(false);
    setError(false);
    setNudgeChannelPreference(null);
    setNudgeChannelLoading(true);
    setNudgeChannelSaving(false);
    setNudgeChannelError(false);
    let current = true;
    void presentation.synchronizePushPermission(false)
      .then((next) => {
        if (current && activeOwnerKey.current === ownedSessionKey) setPermission(next);
      })
      .catch(() => {
        if (current && activeOwnerKey.current === ownedSessionKey) {
          setPermission({ granted: false, canAskAgain: false });
        }
      });
    void loadNudgeChannel();
    return () => {
      current = false;
      nudgeChannelLoadRevision.current += 1;
      nudgeChannelOwner.cancel();
    };
  }, [presentation]);

  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleNotificationJourneyBootstrap(
      intent,
      () => router.replace('/(tabs)/home'),
      lastRecoverySessionKey.current,
    );
  }, [presentation, recovery]);

  useEffect(() => {
    if (!presentation) return;
    const ownedSessionKey = presentation.sessionKey;
    return subscribeNativeAppActive(() => {
      void presentation.synchronizePushPermission(false).then((next) => {
        if (activeOwnerKey.current === ownedSessionKey) setPermission(next);
      }).catch(() => undefined);
    });
  }, [presentation]);

  if (!presentation) return <NotificationJourneyRecoveryView
    i18n={i18n}
    intent={intent}
    onHome={() => router.replace('/(tabs)/home')}
    onRetry={recovery?.retry ?? (() => {
      queueNotificationJourneyBootstrap(intent, lastRecoverySessionKey.current);
      router.replace('/(tabs)/home');
    })}
    state={recovery?.state ?? 'loading'}
  />;
  const activePresentation = presentation;
  const ownsState = ownsNotificationSettingsState(stateOwnerKey, activePresentation.sessionKey);
  const ownedPermission = ownsState ? permission : null;
  const ownedNudgeChannelPreference = ownsState ? nudgeChannelPreference : null;
  const ownedError = ownsState && error;
  const ownedWorking = ownsState && working;
  const ownedNudgeChannelError = ownsState && nudgeChannelError;
  const ownedNudgeChannelLoading = !ownsState || nudgeChannelLoading;
  const ownedNudgeChannelSaving = ownsState && nudgeChannelSaving;

  async function updatePermission() {
    if (!ownsState || !ownedPermission || ownedWorking) return;
    const ownedSessionKey = activePresentation.sessionKey;
    setWorking(true);
    setError(false);
    try {
      if (!ownedPermission.granted && ownedPermission.canAskAgain) {
        const next = await activePresentation.synchronizePushPermission(true);
        if (activeOwnerKey.current === ownedSessionKey) setPermission(next);
      } else {
        await openNativePushSettings();
        const next = await activePresentation.synchronizePushPermission(false);
        if (activeOwnerKey.current === ownedSessionKey) setPermission(next);
      }
    } catch {
      const next = await getNativePushPermission().catch(() => ownedPermission);
      if (activeOwnerKey.current === ownedSessionKey) {
        setPermission(next);
        setError(true);
      }
    } finally {
      if (activeOwnerKey.current === ownedSessionKey) setWorking(false);
    }
  }

  async function updateNudgeChannel(enabled: boolean) {
    if (!ownsState || !ownedNudgeChannelPreference || ownedNudgeChannelSaving) return;
    const ownedSessionKey = activePresentation.sessionKey;
    setNudgeChannelSaving(true);
    setNudgeChannelError(false);
    const result = await nudgeChannelOwner.submit(
      reviewNudgeChannelChange(ownedNudgeChannelPreference, enabled),
      (body, idempotencyKey) => activePresentation.updateNudgeChannelPreference(body, idempotencyKey),
    );
    if (activeOwnerKey.current !== ownedSessionKey) return;
    if (result.kind === 'applied') setNudgeChannelPreference(result.preference);
    else if (result.kind === 'failed') setNudgeChannelError(true);
    if (result.kind !== 'superseded') setNudgeChannelSaving(false);
  }

  const stateKey = ownedPermission?.granted
    ? 'notification.settings.allowed'
    : ownedPermission
      ? 'notification.settings.disabled'
      : 'common.loading';
  const actionKey = ownedWorking
    ? 'notification.settings.working'
    : !ownedPermission?.granted && ownedPermission?.canAskAgain
      ? 'notification.settings.request'
      : 'notification.settings.openSystem';

  return <SettingsShell>
    <SettingsSection footer={i18n.t(ownedError ? 'notification.settings.error' : 'notification.settings.footer')}>
      <SettingsValueRow
        label={i18n.t('notification.settings.permission')}
        value={i18n.t(stateKey)}
      />
    </SettingsSection>
    <SettingsSection footer={i18n.t(ownedNudgeChannelError && ownedNudgeChannelPreference ? 'nudge.channel.saveError' : 'nudge.channel.footer')}>
      {ownedNudgeChannelPreference ? <SettingsSwitchRow
        accessibilityLabel={i18n.t('notification.settings.nudges')}
        accessibilityLiveRegion="polite"
        disabled={ownedNudgeChannelSaving}
        label={i18n.t('notification.settings.nudges')}
        onValueChange={(enabled) => void updateNudgeChannel(enabled)}
        value={ownedNudgeChannelPreference.enabled}
        valueLabel={i18n.t(ownedNudgeChannelSaving
          ? 'nudge.channel.saving'
          : ownedNudgeChannelPreference.enabled ? 'nudge.channel.on' : 'nudge.channel.off')}
      /> : <SettingsValueRow
        label={i18n.t('notification.settings.nudges')}
        value={i18n.t(ownedNudgeChannelLoading ? 'nudge.channel.loading' : 'nudge.channel.loadError')}
      />}
      {ownedNudgeChannelError ? <><SettingsSeparator /><SettingsActionRow
        accessibilityLabel={i18n.t('common.retry')}
        disabled={ownedNudgeChannelSaving}
        label={i18n.t('common.retry')}
        onPress={() => void loadNudgeChannel()}
        tone="default"
      /></> : null}
    </SettingsSection>
    <SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t(actionKey)}
        disabled={!ownedPermission || ownedWorking}
        label={i18n.t(actionKey)}
        onPress={() => void updatePermission()}
        tone="default"
      />
    </SettingsSection>
  </SettingsShell>;
}
