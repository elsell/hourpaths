import { getLocales } from 'expo-localization';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { AccessibilityInfo, findNodeHandle, Pressable, ScrollView, StyleSheet, useWindowDimensions, View } from 'react-native';
import { PathEmoji } from './path-emoji';
import { pathPalette, type PathAppearance } from './path-appearance';
import { createDeviceTranslator } from '../i18n';
import { SortablePathGrid } from './sortable-path-grid';
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
  reorderDisabled?: boolean;
  onReorder?: (section: string, ids: string[]) => void;
  reorderGroup?: (id: string) => string;
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
  reorderDisabled = false,
  onReorder,
  reorderGroup = () => 'manual',
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
  const [dragging, setDragging] = useState(false);
  const [scrollOffset, setScrollOffset] = useState(0);
  const offset = useRef(0);
  const contentHeight = useRef(0);
  const viewport = useRef({ top: 0, height: 0 });
  const fingerY = useRef(0);
  useEffect(() => {
    if (!dragging) return;
    const timer = setInterval(() => {
      const { top, height } = viewport.current;
      const edge = 72;
      const y = fingerY.current - top;
      const speed = y < edge ? -Math.min(16, (edge - y) / 4) : y > height - edge ? Math.min(16, (y - height + edge) / 4) : 0;
      if (!speed || !height) return;
      const next = Math.max(0, Math.min(contentHeight.current - height, offset.current + speed));
      if (next === offset.current) return;
      offset.current = next;
      setScrollOffset(next);
      scroll.current?.scrollTo({ y: next, animated: false });
    }, 16);
    return () => clearInterval(timer);
  }, [dragging]);
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
    scrollEnabled={!dragging}
    scrollEventThrottle={16}
    onScroll={event => { offset.current = event.nativeEvent.contentOffset.y; setScrollOffset(offset.current); }}
    onContentSizeChange={(_width, height) => { contentHeight.current = height; }}
    onLayout={() => scroll.current?.getNativeScrollRef()?.measureInWindow((_x, top, _width, height) => { viewport.current = { top, height }; })}
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
      <SortablePathGrid items={section.items} columns={columns} disabled={reorderDisabled || !onReorder}
        scrollOffset={scrollOffset} groupForID={reorderGroup}
        reorderHint={i18n.t('home.arrange.dragHint')}
        onCommit={ids => onReorder?.(section.key, ids)}
        onDragChange={setDragging} onDragPosition={y => { fingerY.current = y; }}
        onTile={(id, node) => { if (node) tiles.current.set(id, node); else tiles.current.delete(id); }}
        onFocusTarget={(id, node) => { if (node) focusTargets.current.set(id, node); else focusTargets.current.delete(id); }} />
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
