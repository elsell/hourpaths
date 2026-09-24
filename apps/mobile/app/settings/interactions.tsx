import * as Crypto from 'expo-crypto';
import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { createDeviceTranslator } from '../../src/i18n';
import { queueSettingsJourneyBootstrap, scheduleSettingsJourneyBootstrap } from '../../src/settings-journey-route-recovery';
import { ownsSettingsRouteState, settingsRouteOwnerDecision } from '../../src/settings-route-state';
import {
  createInteractionSettingsIntentCoordinator,
  interactionSettingsWithChange,
  type InteractionSetting,
  type InteractionSettings,
} from '../../src/interaction-settings';
import { useSettingsJourneyRouteAncestry } from '../../src/use-settings-journey-route-ancestry';
import { NativeContentUnavailable } from '../../src/ui/native-content-unavailable';
import { SettingsJourneyRecoveryView } from '../../src/ui/settings-journey-recovery-view';
import { useSettingsJourneyRecovery } from '../../src/ui/settings-journey-route-presentation';
import { SettingsActionRow, SettingsSection, SettingsSeparator, SettingsShell, SettingsSwitchRow } from '../../src/ui/settings-list';
import { useSettingsPresentation } from '../../src/ui/settings-presentation';
import { StatusBanner, ThemedText as Text } from '../../src/ui/primitives';
import { mobileTheme } from '../../src/ui/tokens';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'interactions', routeKey: 'settings:interactions' } as const;

export default function InteractionSettingsRoute() {
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
  const [preferences, setPreferences] = useState<InteractionSettings>();
  const [loadFailed, setLoadFailed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);
  const requestRevision = useRef(0);
  const mutationRevision = useRef(0);
  const mutationAdmission = useRef(false);
  const [stateOwnerKey, setStateOwnerKey] = useState<string | null>(null);
  const activeOwnerKey = useRef<string | null>(presentation?.sessionKey ?? null);
  activeOwnerKey.current = presentation?.sessionKey ?? null;
  const intents = useRef(createInteractionSettingsIntentCoordinator(() => Crypto.randomUUID()));

  async function load(ownedPresentation = presentation) {
    if (!ownedPresentation) return;
    const ownedSessionKey = ownedPresentation.sessionKey;
    const revision = ++requestRevision.current;
    ++mutationRevision.current;
    setLoadFailed(false);
    setSaveFailed(false);
    setSaving(false);
    mutationAdmission.current = false;
    try {
      const authoritative = await ownedPresentation.getInteractionSettings();
      if (requestRevision.current !== revision || activeOwnerKey.current !== ownedSessionKey) return;
      intents.current.invalidate();
      setPreferences(authoritative);
      setLoadFailed(false);
    } catch {
      if (requestRevision.current === revision && activeOwnerKey.current === ownedSessionKey) setLoadFailed(true);
    }
  }

  useEffect(() => {
    if (!presentation) return;
    const ownedSessionKey = presentation.sessionKey;
    setStateOwnerKey(ownedSessionKey);
    setPreferences(undefined);
    setLoadFailed(false);
    setSaveFailed(false);
    setSaving(false);
    void load(presentation);
    return () => {
      ++requestRevision.current;
      ++mutationRevision.current;
      intents.current.invalidate();
    };
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
  const ownsState = ownsSettingsRouteState(stateOwnerKey, presentation.sessionKey);
  const ownedPreferences = ownsState ? preferences : undefined;

  async function update(setting: InteractionSetting, enabled: boolean) {
    if (!presentation || !ownedPreferences || mutationAdmission.current) return;
    const ownedPresentation = presentation;
    const ownedSessionKey = ownedPresentation.sessionKey;
    const baseline = ownedPreferences;
    const next = interactionSettingsWithChange(baseline, setting, enabled);
    const intent = intents.current.freeze(next);
    const revision = ++mutationRevision.current;
    mutationAdmission.current = true;
    setPreferences(next);
    setSaveFailed(false);
    setSaving(true);
    try {
      const authoritative = await ownedPresentation.updateInteractionSettings(intent.settings, intent.idempotencyKey);
      if (mutationRevision.current !== revision || activeOwnerKey.current !== ownedSessionKey) return;
      intents.current.complete(intent);
      setPreferences(authoritative);
      setLoadFailed(false);
    } catch {
      if (mutationRevision.current !== revision || activeOwnerKey.current !== ownedSessionKey) return;
      setPreferences(baseline);
      setSaveFailed(true);
    } finally {
      if (mutationRevision.current === revision && activeOwnerKey.current === ownedSessionKey) {
        mutationAdmission.current = false;
        setSaving(false);
      }
    }
  }

  if (!ownedPreferences) {
    return <SettingsShell>
      {loadFailed ? <>
        <NativeContentUnavailable
          description={i18n.t('settings.interactions.unavailableDescription')}
          systemImage="wifi.exclamationmark"
          title={i18n.t('settings.interactions.unavailableHeading')}
        />
        <SettingsSection>
          <SettingsActionRow
            accessibilityLabel={i18n.t('settings.interactions.retry')}
            label={i18n.t('settings.interactions.retry')}
            onPress={() => void load()}
            tone="default"
          />
        </SettingsSection>
      </> : <View
        accessibilityLabel={i18n.t('settings.interactions.loading')}
        accessibilityRole="progressbar"
        style={{ alignItems: 'center', padding: mobileTheme.spacing.xxl }}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      </View>}
    </SettingsShell>;
  }

  return <SettingsShell>
    <SettingsSection footer={i18n.t('settings.interactions.footer')}>
      <SettingsSwitchRow
        accessibilityLabel={i18n.t('settings.interactions.comments')}
        disabled={saving}
        label={i18n.t('settings.interactions.comments')}
        onValueChange={(enabled) => void update('commentsEnabled', enabled)}
        value={ownedPreferences.commentsEnabled}
        valueLabel={i18n.t(ownedPreferences.commentsEnabled ? 'settings.interactions.on' : 'settings.interactions.off')}
      />
      <SettingsSeparator />
      <SettingsSwitchRow
        accessibilityLabel={i18n.t('settings.interactions.reactions')}
        disabled={saving}
        label={i18n.t('settings.interactions.reactions')}
        onValueChange={(enabled) => void update('reactionsEnabled', enabled)}
        value={ownedPreferences.reactionsEnabled}
        valueLabel={i18n.t(ownedPreferences.reactionsEnabled ? 'settings.interactions.on' : 'settings.interactions.off')}
      />
    </SettingsSection>
    {loadFailed ? <>
      <StatusBanner
        text={i18n.t('settings.interactions.unavailableDescription')}
        tone="error"
      />
      <SettingsSection>
        <SettingsActionRow
          accessibilityLabel={i18n.t('settings.interactions.retry')}
          disabled={saving}
          label={i18n.t('settings.interactions.retry')}
          onPress={() => void load()}
          tone="default"
        />
      </SettingsSection>
    </> : null}
    {saveFailed ? <Text accessibilityRole="alert">{i18n.t('settings.interactions.saveError')}</Text> : null}
    {saving ? <Text accessibilityLiveRegion="polite">{i18n.t('settings.interactions.saving')}</Text> : null}
  </SettingsShell>;
}
