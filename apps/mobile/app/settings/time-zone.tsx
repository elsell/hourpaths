import * as Crypto from 'expo-crypto';
import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, InputAccessoryView, Keyboard, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import { createDeviceTranslator } from '../../src/i18n';
import { queueSettingsJourneyBootstrap, scheduleSettingsJourneyBootstrap } from '../../src/settings-journey-route-recovery';
import { ownsSettingsRouteState, settingsRouteOwnerDecision } from '../../src/settings-route-state';
import { createTimeZoneChangeIntentCoordinator, filterTimeZones, type FrozenTimeZoneChangeIntent, type TimeZonePreference } from '../../src/time-zone-settings';
import { useSettingsJourneyRouteAncestry } from '../../src/use-settings-journey-route-ancestry';
import { NativeContentUnavailable } from '../../src/ui/native-content-unavailable';
import { NativeSystemImage } from '../../src/ui/native-system-image';
import { SettingsJourneyRecoveryView } from '../../src/ui/settings-journey-recovery-view';
import { useSettingsJourneyRecovery } from '../../src/ui/settings-journey-route-presentation';
import { SettingsActionRow, SettingsSection, SettingsShell } from '../../src/ui/settings-list';
import { useSettingsPresentation, type SettingsPresentation } from '../../src/ui/settings-presentation';
import { mobileTheme } from '../../src/ui/tokens';

const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'time-zone', routeKey: 'settings:time-zone' } as const;
const keyboardAccessoryID = 'settings-time-zone-keyboard';
const supportedTimeZones = (() => {
  const intl = Intl as typeof Intl & { supportedValuesOf?: (key: 'timeZone') => string[] };
  return intl.supportedValuesOf?.('timeZone') ?? [];
})();

export default function TimeZoneSettingsRoute() {
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
  const [preference, setPreference] = useState<TimeZonePreference>();
  const [query, setQuery] = useState('');
  const [loadFailed, setLoadFailed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveFailed, setSaveFailed] = useState(false);
  const requestRevision = useRef(0);
  const mutationRevision = useRef(0);
  const mutationAdmission = useRef(false);
  const [stateOwnerKey, setStateOwnerKey] = useState<string | null>(null);
  const activeOwnerKey = useRef<string | null>(presentation?.sessionKey ?? null);
  activeOwnerKey.current = presentation?.sessionKey ?? null;
  const intents = useRef(createTimeZoneChangeIntentCoordinator(() => Crypto.randomUUID()));

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
      const authoritative = await ownedPresentation.getConfiguredTimeZone();
      if (requestRevision.current !== revision || activeOwnerKey.current !== ownedSessionKey) return;
      intents.current.invalidate();
      setPreference(authoritative);
    } catch {
      if (requestRevision.current === revision && activeOwnerKey.current === ownedSessionKey) setLoadFailed(true);
    }
  }

  useEffect(() => {
    if (!presentation) return;
    const ownedSessionKey = presentation.sessionKey;
    setStateOwnerKey(ownedSessionKey);
    setPreference(undefined);
    setQuery('');
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

  const timeZones = useMemo(
    () => filterTimeZones(supportedTimeZones, query, preference?.timeZone ?? ''),
    [preference?.timeZone, query],
  );

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
  const ownedPreference = ownsState ? preference : undefined;

  async function save(
    changeIntent: FrozenTimeZoneChangeIntent,
    ownedPresentation: SettingsPresentation,
    ownedSessionKey: string,
  ) {
    if (mutationAdmission.current || activeOwnerKey.current !== ownedSessionKey ||
      preference?.timeZone !== changeIntent.reviewedTimeZone) return;
    mutationAdmission.current = true;
    const revision = ++mutationRevision.current;
    setSaveFailed(false);
    setSaving(true);
    try {
      const authoritative = await ownedPresentation.updateConfiguredTimeZone(changeIntent);
      if (mutationRevision.current !== revision || activeOwnerKey.current !== ownedSessionKey) return;
      intents.current.complete(changeIntent);
      setPreference(authoritative);
      setQuery('');
    } catch {
      if (mutationRevision.current === revision && activeOwnerKey.current === ownedSessionKey) setSaveFailed(true);
    } finally {
      if (mutationRevision.current === revision && activeOwnerKey.current === ownedSessionKey) {
        mutationAdmission.current = false;
        setSaving(false);
      }
    }
  }

  function review(proposedTimeZone: string) {
    if (!presentation || !ownedPreference || saving || proposedTimeZone === ownedPreference.timeZone) return;
    const ownedPresentation = presentation;
    const ownedSessionKey = ownedPresentation.sessionKey;
    const changeIntent = intents.current.freeze(ownedPreference.timeZone, proposedTimeZone);
    Alert.alert(
      i18n.t('settings.timeZone.warningTitle'),
      i18n.t('settings.timeZone.warning', { current: ownedPreference.timeZone, proposed: proposedTimeZone }),
      [
        { style: 'cancel', text: i18n.t('settings.timeZone.cancel') },
        {
          text: i18n.t('settings.timeZone.confirm'),
          onPress: () => void save(changeIntent, ownedPresentation, ownedSessionKey),
        },
      ],
    );
  }

  if (!ownedPreference) {
    return <SettingsShell>
      {loadFailed ? <>
        <NativeContentUnavailable
          description={i18n.t('settings.timeZone.unavailableDescription')}
          systemImage="wifi.exclamationmark"
          title={i18n.t('settings.timeZone.unavailableHeading')}
        />
        <SettingsSection>
          <SettingsActionRow
            accessibilityLabel={i18n.t('settings.timeZone.retry')}
            label={i18n.t('settings.timeZone.retry')}
            onPress={() => void load()}
            tone="default"
          />
        </SettingsSection>
      </> : <View
        accessibilityLabel={i18n.t('settings.timeZone.loading')}
        accessibilityRole="progressbar"
        style={styles.loading}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      </View>}
    </SettingsShell>;
  }

  return <View style={styles.screen}>
    <FlatList
      automaticallyAdjustContentInsets
      contentContainerStyle={styles.content}
      contentInsetAdjustmentBehavior="automatic"
      data={timeZones}
      ItemSeparatorComponent={() => <View style={styles.separator} />}
      keyboardDismissMode="on-drag"
      keyboardShouldPersistTaps="handled"
      keyExtractor={(timeZone) => timeZone}
      ListEmptyComponent={<Text style={styles.empty}>{i18n.t('settings.timeZone.empty')}</Text>}
      ListHeaderComponent={<View style={styles.header}>
        <Text style={styles.current}>{i18n.t('settings.timeZone.current', { timeZone: ownedPreference.timeZone })}</Text>
        <TextInput
          accessibilityLabel={i18n.t('settings.timeZone.searchPlaceholder')}
          accessibilityRole="search"
          autoCapitalize="none"
          autoCorrect={false}
          clearButtonMode="while-editing"
          inputAccessoryViewID={keyboardAccessoryID}
          onChangeText={setQuery}
          placeholder={i18n.t('settings.timeZone.searchPlaceholder')}
          placeholderTextColor={mobileTheme.colors.textMuted}
          returnKeyType="search"
          style={styles.search}
          value={query}
        />
        {saveFailed ? <Text accessibilityRole="alert" style={styles.error}>{i18n.t('settings.timeZone.saveError')}</Text> : null}
        {saving ? <Text accessibilityLiveRegion="polite" style={styles.status}>{i18n.t('settings.timeZone.saving')}</Text> : null}
      </View>}
      renderItem={({ item }) => {
        const selected = item === ownedPreference.timeZone;
        return <Pressable
          accessibilityRole="button"
          accessibilityState={{ selected: selected }}
          disabled={saving}
          onPress={() => review(item)}
          style={({ pressed }) => [styles.row, pressed ? styles.rowPressed : null]}
        >
          <Text style={styles.rowLabel}>{item}</Text>
          {selected ? <NativeSystemImage systemName="checkmark" /> : null}
        </Pressable>;
      }}
    />
    <InputAccessoryView nativeID={keyboardAccessoryID}>
      <View style={styles.keyboardToolbar}>
        <Pressable accessibilityRole="button" onPress={Keyboard.dismiss} style={styles.keyboardDone}>
          <Text style={styles.keyboardDoneLabel}>{i18n.t('settings.timeZone.keyboardDone')}</Text>
        </Pressable>
      </View>
    </InputAccessoryView>
  </View>;
}

const styles = StyleSheet.create({
  content: { padding: mobileTheme.spacing.md, paddingBottom: mobileTheme.spacing.xxl },
  current: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20 },
  empty: { color: mobileTheme.colors.textMuted, fontSize: 17, padding: mobileTheme.spacing.lg, textAlign: 'center' },
  error: { color: mobileTheme.colors.error, fontSize: 15, lineHeight: 20 },
  header: { gap: mobileTheme.spacing.sm, paddingBottom: mobileTheme.spacing.md },
  loading: { alignItems: 'center', padding: mobileTheme.spacing.xxl },
  row: { alignItems: 'center', backgroundColor: mobileTheme.colors.surface, flexDirection: 'row', justifyContent: 'space-between', minHeight: 50, paddingHorizontal: mobileTheme.spacing.md, paddingVertical: mobileTheme.spacing.sm },
  rowLabel: { color: mobileTheme.colors.text, flex: 1, fontSize: 17, lineHeight: 22 },
  rowPressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  screen: { backgroundColor: mobileTheme.colors.background, flex: 1 },
  search: { backgroundColor: mobileTheme.colors.surfaceRaised, borderRadius: mobileTheme.radii.md, color: mobileTheme.colors.text, fontSize: 17, minHeight: 48, paddingHorizontal: mobileTheme.spacing.md },
  separator: { backgroundColor: mobileTheme.colors.border, height: StyleSheet.hairlineWidth, marginLeft: mobileTheme.spacing.md },
  status: { color: mobileTheme.colors.textMuted, fontSize: 15, lineHeight: 20 },
  keyboardDone: { alignItems: 'center', justifyContent: 'center', minHeight: mobileTheme.sizes.minimumTouchTarget, paddingHorizontal: mobileTheme.spacing.md },
  keyboardDoneLabel: { color: mobileTheme.colors.accent, fontSize: 17, fontWeight: '600' },
  keyboardToolbar: { alignItems: 'flex-end', backgroundColor: mobileTheme.colors.surface, borderTopColor: mobileTheme.colors.border, borderTopWidth: StyleSheet.hairlineWidth },
});
