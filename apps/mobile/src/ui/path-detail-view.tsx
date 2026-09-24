import { getLocales } from 'expo-localization';
import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { formatCompactDuration } from './compact-duration';
import { NativePrimaryButton } from './native-primary-button';
import { SectionHeading, StatusBanner, Surface, ThemedText as Text } from './primitives';
import { SettingsNavigationRow, SettingsSection } from './settings-list';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export function PathDetailView({
  accumulatedSeconds,
  actionDisabled,
  archived,
  busy,
  canTrackTime,
  comparisonFirst = false,
  intervalProgress,
  onAddActivity,
  onOpenHistory,
  overallProgress,
  participantComparison,
}: {
  accumulatedSeconds?: number;
  actionDisabled: boolean;
  archived: boolean;
  busy: boolean;
  canTrackTime: boolean;
  comparisonFirst?: boolean;
  intervalProgress?: ReactNode;
  onAddActivity: () => void;
  onOpenHistory: () => void;
  overallProgress?: ReactNode;
  participantComparison?: ReactNode;
}) {
  const accumulated = accumulatedSeconds === undefined
    ? undefined
    : formatCompactDuration(accumulatedSeconds, i18n);
  const hasGoals = Boolean(intervalProgress || overallProgress);
  const comparison = participantComparison ? <View style={styles.comparison}>
    <SectionHeading>{i18n.t('pathMembers.heading')}</SectionHeading>
    {participantComparison}
  </View> : null;

  return <View style={styles.stack}>
    {archived
      ? <Surface>
        <Text accessibilityRole="alert" style={styles.archived}>
          {i18n.t('pathArchive.readOnly')}
        </Text>
      </Surface>
      : null}

    {comparisonFirst ? comparison : null}

    {accumulated ? <Surface>
      <Text style={styles.label}>{i18n.t('pathDetails.totalTime')}</Text>
      <Text
        accessibilityLabel={i18n.t('pathDetails.totalTimeValue', { duration: accumulated })}
        style={styles.total}
      >
        {accumulated}
      </Text>
    </Surface> : null}

    {hasGoals ? <View style={styles.goals}>
      <SectionHeading>{i18n.t('pathDetails.goals')}</SectionHeading>
      {intervalProgress}
      {overallProgress}
    </View> : null}

    {!archived && canTrackTime ? <NativePrimaryButton
      disabled={actionDisabled}
      label={i18n.t('activity.add')}
      onPress={onAddActivity}
      systemImage="plus"
    /> : null}
    <SettingsSection>
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('pathDetails.openHistory')}
        label={i18n.t('pathDetails.openHistory')}
        onPress={onOpenHistory}
      />
    </SettingsSection>
    {!comparisonFirst ? comparison : null}
    {busy ? <StatusBanner text={i18n.t('common.loading')} /> : null}
  </View>;
}

const styles = StyleSheet.create({
  archived: {
    color: mobileTheme.colors.textMuted,
  },
  comparison: {
    gap: mobileTheme.spacing.sm,
  },
  goals: {
    gap: mobileTheme.spacing.sm,
  },
  label: {
    ...mobileTheme.typography.caption,
    color: mobileTheme.colors.textMuted,
  },
  stack: {
    gap: mobileTheme.spacing.md,
  },
  total: {
    ...mobileTheme.typography.title,
    marginTop: mobileTheme.spacing.xxs,
  },
});
