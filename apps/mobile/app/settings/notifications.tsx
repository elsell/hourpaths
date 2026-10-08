import * as Crypto from 'expo-crypto';
import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { Fragment, useEffect, useRef, useState } from 'react';
import {
  createNotificationChannelOperationOwner,
  type NotificationChannelPreference,
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
  const [notificationChannelPreference, setNotificationChannelPreference] = useState<NotificationChannelPreference[] | null>(null);
  const [notificationChannelLoading, setNotificationChannelLoading] = useState(true);
  const [notificationChannelSaving, setNotificationChannelSaving] = useState(false);
  const [savingChannel, setSavingChannel] = useState<string | null>(null);
  const [notificationChannelError, setNotificationChannelError] = useState(false);
  const [notificationChannelOwner] = useState(() => createNotificationChannelOperationOwner(() => Crypto.randomUUID()));
  const notificationChannelLoadRevision = useRef(0);
  const activeOwnerKey = useRef<string | null>(null);
  activeOwnerKey.current = presentation?.sessionKey ?? null;

  async function loadNotificationChannel() {
    if (!presentation) return;
    const ownedSessionKey = presentation.sessionKey;
    const revision = ++notificationChannelLoadRevision.current;
    setNotificationChannelLoading(true);
    setNotificationChannelError(false);
    try {
      const preference = await presentation.getNotificationChannels();
      if (revision === notificationChannelLoadRevision.current && activeOwnerKey.current === ownedSessionKey) {
        setNotificationChannelPreference(preference);
      }
    } catch {
      if (revision === notificationChannelLoadRevision.current && activeOwnerKey.current === ownedSessionKey) {
        setNotificationChannelError(true);
      }
    } finally {
      if (revision === notificationChannelLoadRevision.current && activeOwnerKey.current === ownedSessionKey) {
        setNotificationChannelLoading(false);
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
    setNotificationChannelPreference(null);
    setNotificationChannelLoading(true);
    setNotificationChannelSaving(false);
    setSavingChannel(null);
    setNotificationChannelError(false);
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
    void loadNotificationChannel();
    return () => {
      current = false;
      notificationChannelLoadRevision.current += 1;
      notificationChannelOwner.cancel();
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
  const ownedNotificationChannelPreference = ownsState ? notificationChannelPreference : null;
  const ownedError = ownsState && error;
  const ownedWorking = ownsState && working;
  const ownedNotificationChannelError = ownsState && notificationChannelError;
  const ownedNotificationChannelLoading = !ownsState || notificationChannelLoading;
  const ownedNotificationChannelSaving = ownsState && notificationChannelSaving;

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

  async function updateNotificationChannel(value: NotificationChannelPreference, enabled: boolean) {
    if (!ownsState || !ownedNotificationChannelPreference || ownedNotificationChannelSaving || value.enabled === enabled) return;
    const ownedSessionKey = activePresentation.sessionKey;
    setNotificationChannelSaving(true);
    setSavingChannel(value.channel);
    setNotificationChannelError(false);
    const result = await notificationChannelOwner.submit(
      { ...value, enabled },
      (next, idempotencyKey) => activePresentation.updateNotificationChannel(next, idempotencyKey),
    );
    if (activeOwnerKey.current !== ownedSessionKey) return;
    if (result.kind === 'applied') setNotificationChannelPreference(rows => rows?.map(row => row.channel === result.preference.channel ? result.preference : row) ?? null);
    else if (result.kind === 'failed') setNotificationChannelError(true);
    if (result.kind !== 'superseded') setNotificationChannelSaving(false);
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
    <SettingsSection title={i18n.t('notification.channels.heading')} footer={i18n.t(ownedNotificationChannelError && ownedNotificationChannelPreference ? 'notification.channels.saveError' : 'notification.channels.footer')}>
      {ownedNotificationChannelPreference ? ownedNotificationChannelPreference.map((row, index) => <Fragment key={row.channel}>
        {index > 0 && <SettingsSeparator />}
        <SettingsSwitchRow
          accessibilityLabel={i18n.t(`notification.channel.${row.channel}`)}
          accessibilityLiveRegion="polite"
          disabled={ownedNotificationChannelSaving}
          label={i18n.t(`notification.channel.${row.channel}`)}
          onValueChange={enabled => void updateNotificationChannel(row, enabled)}
          value={row.enabled}
          valueLabel={i18n.t(ownedNotificationChannelSaving && savingChannel === row.channel ? 'notification.channels.saving' : row.enabled ? 'nudge.channel.on' : 'nudge.channel.off')}
        />
      </Fragment>) : <SettingsValueRow label={i18n.t('notification.channels.heading')} value={i18n.t(ownedNotificationChannelLoading ? 'notification.channels.loading' : 'notification.channels.loadError')} />}
      {ownedNotificationChannelError ? <><SettingsSeparator /><SettingsActionRow
        accessibilityLabel={i18n.t('common.retry')}
        disabled={ownedNotificationChannelSaving}
        label={i18n.t('common.retry')}
        onPress={() => void loadNotificationChannel()}
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
