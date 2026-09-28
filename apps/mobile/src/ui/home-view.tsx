import { getLocales } from 'expo-localization';
import { cloneElement, isValidElement, useRef, type ReactNode } from 'react';
import { AccessibilityInfo, findNodeHandle, Pressable, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native';
import { PathEmoji } from './path-emoji';
import { pathPalette, type PathAppearance } from './path-appearance';
import { createDeviceTranslator } from '../i18n';
import type { PathCardProps } from './path-card';
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
  running?: readonly { id: string; name: string; appearance: PathAppearance }[];
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
  running = [],
  filterControl,
  notice,
  onClearFilter,
  onCreate,
  onRetry,
  presentation,
  sections,
}: HomeViewProps) {
  const scroll = useRef<ScrollView>(null);
  const content = useRef<View>(null);
  const tiles = useRef(new Map<string, View>());
  const focusTargets = useRef(new Map<string, View>());
  const { width, fontScale } = useWindowDimensions();
  const columns = width >= 360 && fontScale <= 1.3 ? 2 : 1;
  async function jumpToPath(id: string) {
    const tile = tiles.current.get(id);
    if (!tile || !content.current) return;
    const reducedMotion = await AccessibilityInfo.isReduceMotionEnabled();
    tile.measureLayout(content.current, (_x, y) => {
      scroll.current?.scrollTo({ y: Math.max(0, y - mobileTheme.spacing.sm), animated: !reducedMotion });
      const handle = findNodeHandle(focusTargets.current.get(id) ?? tile);
      if (handle) AccessibilityInfo.setAccessibilityFocus(handle);
    });
  }

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
    ref={scroll}
    automaticallyAdjustContentInsets
    contentContainerStyle={styles.content}
    contentInsetAdjustmentBehavior="automatic"
  >
    <View ref={content} collapsable={false} style={styles.collection}>
    {notice}
    {filterControl}
    {running.length > 0 ? <View style={styles.running}>
      <Text accessibilityRole="header" style={styles.runningTitle}>{i18n.t('home.activeTimersHeading')}</Text>
      <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.shortcuts}>
        {running.map((path) => <Pressable key={path.id}
          accessibilityRole="button"
          accessibilityLabel={i18n.t('home.jumpToPath', { pathName: path.name })}
          onPress={() => void jumpToPath(path.id)}
          style={({ pressed }) => [styles.shortcut, { backgroundColor: pathPalette[path.appearance.color].background }, pressed ? { opacity: 0.65 } : null]}>
          <PathEmoji emoji={path.appearance.emoji} size={mobileTheme.spacing.xl} />
          <Text style={[styles.shortcutText, { color: pathPalette[path.appearance.color].foreground }]}>{path.name}</Text>
        </Pressable>)}
      </ScrollView>
    </View> : null}
    {sections.map((section) => <View key={section.key} style={styles.section}>
      <Text accessibilityRole="header" style={styles.sectionTitle}>{section.title}</Text>
      <View style={styles.rows}>{section.items.map((item, index) => {
        const id = isValidElement(item) && item.key !== null ? String(item.key) : `${section.key}-${index}`;
        return <View key={id} collapsable={false}
          ref={(node) => { if (node) tiles.current.set(id, node); else tiles.current.delete(id); }}
          style={{ width: columns === 2 ? '48.5%' : '100%' }}>{isValidElement<PathCardProps>(item) ? cloneElement(item, { onFocusTarget: (node) => { if (node) focusTargets.current.set(id, node); else focusTargets.current.delete(id); } }) : item}</View>;
      })}</View>
    </View>)}
    </View>
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
    paddingTop: mobileTheme.spacing.xs,
  },
  section: {
    gap: mobileTheme.spacing.xs,
  },
  rows: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', rowGap: mobileTheme.spacing.sm },
  collection: { gap: mobileTheme.spacing.sm },
  running: { backgroundColor: mobileTheme.colors.surface, borderRadius: mobileTheme.radii.lg, borderCurve: 'continuous', padding: mobileTheme.spacing.sm, gap: mobileTheme.spacing.xs },
  runningTitle: { ...mobileTheme.typography.body, fontWeight: '600' },
  shortcuts: { gap: mobileTheme.spacing.xs },
  shortcut: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.xs, borderRadius: mobileTheme.radii.md, minHeight: mobileTheme.sizes.minimumTouchTarget, justifyContent: 'center', paddingHorizontal: mobileTheme.spacing.sm },
  shortcutText: { ...mobileTheme.typography.body, fontWeight: '600' },
  stateContent: {
    flexGrow: 1,
    justifyContent: 'center',
  },
});
