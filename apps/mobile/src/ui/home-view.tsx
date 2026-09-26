import { getLocales } from 'expo-localization';
import type { ReactNode } from 'react';
import { ScrollView, StyleSheet, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import type { HomePresentation } from './home-presentation';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativePrimaryButton } from './native-primary-button';
import { StatusBanner, ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

export type HomeViewSection = Readonly<{
  key: string;
  items: readonly ReactNode[];
  title: string;
}>;

export type HomeViewProps = {
  filterControl?: ReactNode;
  notice?: ReactNode;
  onClearFilter: () => void;
  onCreate: () => void;
  onRetry: () => void;
  presentation: HomePresentation;
  sections: readonly HomeViewSection[];
};

function StateScroll({ children }: { children: ReactNode }) {
  return <ScrollView
    automaticallyAdjustContentInsets
    contentContainerStyle={[styles.content, styles.stateContent]}
    contentInsetAdjustmentBehavior="automatic"
  >{children}</ScrollView>;
}

export function HomeView({
  filterControl,
  notice,
  onClearFilter,
  onCreate,
  onRetry,
  presentation,
  sections,
}: HomeViewProps) {
  if (presentation.kind === 'loading') return <StateScroll>
    <StatusBanner text={i18n.t('home.loading')} />
  </StateScroll>;

  if (presentation.kind === 'offline') return <StateScroll>
    <NativeContentUnavailable
      description={i18n.t('home.offlineExplanation')}
      systemImage="wifi.slash"
      title={i18n.t('home.offlineHeading')}
    />
    <NativePrimaryButton fullWidth label={i18n.t('common.retry')} onPress={onRetry} systemImage="arrow.clockwise" />
  </StateScroll>;

  if (presentation.kind === 'error') return <StateScroll>
    <NativeContentUnavailable
      description={i18n.t('home.errorExplanation')}
      systemImage="exclamationmark.triangle"
      title={i18n.t('home.errorHeading')}
    />
    <NativePrimaryButton fullWidth label={i18n.t('common.retry')} onPress={onRetry} systemImage="arrow.clockwise" />
  </StateScroll>;

  if (presentation.kind === 'filtered-empty') return <StateScroll>
    {notice}
    {filterControl}
    <NativeContentUnavailable
      description={i18n.t('home.filteredEmptyExplanation')}
      systemImage="line.3.horizontal.decrease.circle"
      title={i18n.t('home.filteredEmptyHeading')}
    />
    <NativePrimaryButton label={i18n.t('home.clearFilter')} onPress={onClearFilter} variant="plain" />
  </StateScroll>;

  if (presentation.kind === 'empty') return <StateScroll>
    {notice}
    <NativeContentUnavailable
      description={i18n.t(presentation.mode === 'active' ? 'home.empty.explanation' : 'home.archivedEmpty')}
      systemImage={presentation.mode === 'active' ? 'figure.walk' : 'archivebox'}
      title={i18n.t(presentation.mode === 'active' ? 'home.empty.heading' : 'home.archivedPaths')}
    />
    {presentation.mode === 'active' ? <NativePrimaryButton
      fullWidth
      label={i18n.t('home.createPath')}
      onPress={onCreate}
      systemImage="plus"
    /> : null}
  </StateScroll>;

  return <ScrollView
    automaticallyAdjustContentInsets
    contentContainerStyle={styles.content}
    contentInsetAdjustmentBehavior="automatic"
  >
    {notice}
    {filterControl}
    {sections.map((section) => <View key={section.key} style={styles.section}>
      <Text accessibilityRole="header" style={styles.sectionTitle}>{section.title}</Text>
      <View style={styles.rows}>{section.items}</View>
    </View>)}
  </ScrollView>;
}

const styles = StyleSheet.create({
  content: {
    gap: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  sectionTitle: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingTop: mobileTheme.spacing.md,
  },
  section: {
    gap: mobileTheme.spacing.xs,
  },
  rows: {
    gap: 0,
  },
  stateContent: {
    flexGrow: 1,
    justifyContent: 'center',
  },
});
