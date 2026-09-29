import { NUDGE_AUDIENCES, type NudgeAudience, type NudgeAudiencePreference } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { StyleSheet, View } from 'react-native';
import { NativeRouteRecoveryView } from './native-route-recovery-view';
import { ThemedText as Text } from './primitives';
import { SettingsChoiceRow } from './settings-choice-row';
import { SettingsSection, SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

export function PathNudgeSettingsView({
  busy,
  error,
  i18n,
  loading,
  onGoHome,
  onRetry,
  onSelect,
  preference,
  recoveryState,
}: {
  busy: boolean;
  error: boolean;
  i18n: Translator;
  loading: boolean;
  onGoHome?: () => void;
  onRetry: () => void;
  onSelect: (audience: NudgeAudience) => void;
  preference: NudgeAudiencePreference | null;
  recoveryState?: 'loading' | 'offline' | 'unavailable';
}) {
  if (!preference) return <NativeRouteRecoveryView
    onGoHome={onGoHome}
    onRetry={loading || recoveryState === 'loading' ? undefined : onRetry}
    state={recoveryState ?? (loading ? 'loading' : 'unavailable')}
  />;

  return <View style={styles.stack}>
    <View accessibilityLabel={i18n.t('nudge.audience.label')} accessibilityRole="radiogroup">
    <SettingsSection footer={i18n.t('nudge.audience.footer')}>
      {NUDGE_AUDIENCES.map((audience, index) => {
        const selected = preference.audience === audience;
        return <View key={audience}>
          {index > 0 ? <SettingsSeparator /> : null}
          <SettingsChoiceRow
            label={i18n.t(`nudge.audience.${audience}`)}
            selected={selected}
            disabled={busy}
            onPress={() => { if (!selected) onSelect(audience); }}
          />
        </View>;
      })}
    </SettingsSection>
    </View>
    {busy ? <Text accessibilityLiveRegion="polite" style={styles.status}>{i18n.t('nudge.audience.saving')}</Text> : null}
    {error ? <Text accessibilityRole="alert" style={styles.error}>{i18n.t('nudge.audience.saveError')}</Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  error: { color: mobileTheme.colors.error, fontSize: 15, lineHeight: 20 },
  stack: { gap: mobileTheme.spacing.sm },
  status: { color: mobileTheme.colors.textMuted, fontSize: 13, lineHeight: 18, textAlign: 'center' },
});
