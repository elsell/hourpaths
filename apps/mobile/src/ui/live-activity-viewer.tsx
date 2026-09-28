import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Animated, Modal, Pressable, ScrollView, StatusBar, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { adjacentLiveActivity, projectLiveActivity, type LiveActivityPage } from '../live-activity-pages';
import { formatGoalDuration, formatSessionClock } from './compact-duration';
import { pathPalette } from './path-appearance';
import { PlatformSymbol } from './platform-symbol';
import { PathEmoji } from './path-emoji';
import { SocialProfileAvatar } from './social-profile-avatar';
import { mobileTheme } from './tokens';

export type LiveActivityViewerProps = {
  pages: readonly LiveActivityPage[];
  initialTimerId: string;
  translator: Translator;
  onClose: () => void;
  onRefresh?: () => void;
  onOpenProfile?: (personId: string) => void;
};

export function LiveActivityViewer({ pages, initialTimerId, translator, onClose, onRefresh, onOpenProfile }: LiveActivityViewerProps) {
  const [selected, setSelected] = useState(initialTimerId);
  const [now, setNow] = useState(Date.now);
  const [reduceMotion, setReduceMotion] = useState(true);
  const [transition, setTransition] = useState<{ previous: LiveActivityPage; direction: -1 | 1 }>();
  const motion = useRef(new Animated.Value(1)).current;
  const busy = useRef(false);
  const requestedInterval = useRef('');
  const { width } = useWindowDimensions();
  const page = pages.find(candidate => candidate.timerId === selected);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    let live = true;
    void AccessibilityInfo.isReduceMotionEnabled().then(value => { if (live) setReduceMotion(value); });
    const listener = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => { live = false; clearInterval(timer); listener.remove(); motion.stopAnimation(); };
  }, [motion]);
  useEffect(() => { if (!page) onClose(); }, [page, onClose]);
  useEffect(() => {
    if (!page || !projectLiveActivity(page, now).goalExpired) return;
    const key = `${page.timerId}:${page.goal?.intervalEnd}`;
    if (requestedInterval.current !== key) { requestedInterval.current = key; onRefresh?.(); }
  }, [page, now, onRefresh]);
  if (!page) return null;
  const currentPage = page;
  function stepPage(direction: -1 | 1) {
    if (busy.current) return;
    const next = adjacentLiveActivity(pages, selected, direction);
    if (!next) { onClose(); return; }
    if (next === selected) return;
    const nextPage = pages.find(candidate => candidate.timerId === next);
    if (reduceMotion || nextPage?.personId === currentPage.personId) { setSelected(next); return; }
    busy.current = true;
    motion.setValue(0);
    setTransition({ previous: currentPage, direction });
    setSelected(next);
    Animated.timing(motion, { toValue: 1, duration: 230, useNativeDriver: true }).start(() => {
      busy.current = false;
      setTransition(undefined);
    });
  }
  const tone = pathPalette[page.appearance.color];
  return <Modal visible animationType="none" presentationStyle="fullScreen" statusBarTranslucent navigationBarTranslucent onRequestClose={onClose}>
    <SafeAreaProvider>
      <StatusBar barStyle="dark-content" backgroundColor={tone.background} />
      <View accessibilityViewIsModal onAccessibilityEscape={onClose} style={[styles.screen, { backgroundColor: tone.background }]}>
        {transition ? <Animated.View pointerEvents="none" accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={[StyleSheet.absoluteFill, { transform: [{ translateX: motion.interpolate({ inputRange: [0, 1], outputRange: [0, -width * transition.direction] }) }] }]}>
          <ActivityPage page={transition.previous} pages={pages} now={now} translator={translator} onClose={onClose} />
        </Animated.View> : null}
        <Animated.View style={[styles.screen, transition ? { transform: [{ translateX: motion.interpolate({ inputRange: [0, 1], outputRange: [width * transition.direction, 0] }) }] } : null]}>
          <ActivityPage page={page} pages={pages} now={now} translator={translator} onClose={onClose} onOpenProfile={onOpenProfile} stepPage={stepPage} />
        </Animated.View>
      </View>
    </SafeAreaProvider>
  </Modal>;
}

function ActivityPage({ page, pages, now, translator, onClose, onOpenProfile, stepPage }: {
  page: LiveActivityPage; pages: readonly LiveActivityPage[]; now: number; translator: Translator;
  onClose: () => void; onOpenProfile?: (personId: string) => void; stepPage?: (direction: -1 | 1) => void;
}) {
  const touchStart = useRef<{ x: number; y: number } | undefined>(undefined);
  const { width: viewportWidth } = useWindowDimensions();
  const tone = pathPalette[page.appearance.color];
  const personPages = pages.filter(candidate => candidate.personId === page.personId);
  const index = personPages.findIndex(candidate => candidate.timerId === page.timerId);
  const projection = projectLiveActivity(page, now);
  const foreground = { color: tone.foreground };
  const goal = (label: string, seconds: number, target: number) => {
    const percentage = Math.min(100, Math.max(0, seconds / Math.max(1, target) * 100));
    const progressWidth = `${percentage}%` as `${number}%`;
    return <View style={styles.goal}>
    <Text style={[styles.goalLabel, foreground]}>{label}</Text>
    <Text style={[styles.goalTotal, foreground]}>{translator.t('social.live.progress', { current: formatGoalDuration(seconds, translator), target: formatGoalDuration(target, translator) })}</Text>
    <View accessible accessibilityRole="progressbar" accessibilityLabel={label} accessibilityValue={{ min: 0, max: Math.max(1, target), now: Math.min(Math.max(1, target), seconds) }} style={[styles.track, { backgroundColor: tone.track }]}>
      <View style={[styles.fill, { backgroundColor: tone.accent, width: progressWidth }]} />
    </View>
  </View>;
  };
  return <SafeAreaView style={[styles.screen, { backgroundColor: tone.background }]}>
    <View style={styles.header}>
      <View accessible accessibilityLabel={translator.t('social.live.pagePosition', { current: index + 1, total: personPages.length })} style={styles.segments}>
        {personPages.map((candidate, position) => <View key={candidate.timerId} style={[styles.segment, { backgroundColor: position <= index ? tone.accent : tone.track }]} />)}
      </View>
      <View style={styles.identityRow}>
        <Pressable disabled={!onOpenProfile} onPress={() => onOpenProfile?.(page.personId)} accessibilityRole={onOpenProfile ? 'button' : undefined} style={styles.identity}>
          <SocialProfileAvatar accessibilityLabel={page.displayName} profilePictureURL={page.profilePictureURL} size={40} />
          <View style={styles.person}><Text numberOfLines={1} style={[styles.name, foreground]}>{page.displayName}</Text><Text numberOfLines={1} style={[styles.username, foreground]}>{page.username}</Text></View>
        </Pressable>
        <Text style={[styles.live, { color: tone.accent }]}>{translator.t('social.live.live')}</Text>
        <Pressable accessibilityLabel={translator.t('social.live.close')} accessibilityRole="button" onPress={onClose} style={styles.close}><PlatformSymbol systemName="xmark" color={tone.foreground} /></Pressable>
      </View>
    </View>
    <ScrollView contentContainerStyle={styles.body}
      onTouchStart={event => { touchStart.current = { x: event.nativeEvent.pageX, y: event.nativeEvent.pageY }; }}
      onTouchCancel={() => { touchStart.current = undefined; }}
      onScrollBeginDrag={() => { touchStart.current = undefined; }}
      onTouchEnd={event => {
        const start = touchStart.current;
        touchStart.current = undefined;
        if (stepPage && start && Math.abs(event.nativeEvent.pageX - start.x) < 10 && Math.abs(event.nativeEvent.pageY - start.y) < 10) stepPage(start.x < viewportWidth / 2 ? -1 : 1);
      }}>
      <View style={styles.content} accessible accessibilityRole="adjustable" accessibilityActions={[{ name: 'increment', label: translator.t('social.live.next') }, { name: 'decrement', label: translator.t('social.live.previous') }]} onAccessibilityAction={event => stepPage?.(event.nativeEvent.actionName === 'decrement' ? -1 : 1)}>
        <PathEmoji emoji={page.appearance.emoji} size={96} />
        <Text style={[styles.pathName, foreground]}>{page.pathName}</Text>
        <Text style={[styles.sessionLabel, foreground]}>{translator.t('social.live.session')}</Text>
        <Text adjustsFontSizeToFit numberOfLines={1} style={[styles.clock, foreground]}>{formatSessionClock(projection.sessionSeconds, translator)}</Text>
        {page.goal && projection.goalSeconds !== undefined ? goal(page.goal.label, projection.goalSeconds, page.goal.targetSeconds) : null}
        {page.overallGoal ? goal(page.overallGoal.label, page.overallGoal.recordedSeconds + projection.sessionSeconds, page.overallGoal.targetSeconds) : null}
        {page.goal || page.overallGoal ? <Text style={[styles.disclaimer, foreground]}>{translator.t(projection.goalExpired ? 'social.live.refreshing' : 'social.live.includingSession')}</Text> : null}
      </View>
    </ScrollView>
  </SafeAreaView>;
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  header: { paddingHorizontal: mobileTheme.spacing.md, paddingTop: mobileTheme.spacing.xs },
  segments: { flexDirection: 'row', gap: mobileTheme.spacing.xxs, marginBottom: mobileTheme.spacing.sm },
  segment: { flex: 1, height: 3, borderRadius: mobileTheme.radii.pill },
  identityRow: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.xs },
  identity: { flex: 1, flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.sm, minHeight: mobileTheme.sizes.minimumTouchTarget },
  person: { flex: 1 },
  name: { ...mobileTheme.typography.body, fontWeight: '700' },
  username: { ...mobileTheme.typography.caption },
  live: { ...mobileTheme.typography.caption, fontWeight: '700' },
  close: { height: mobileTheme.sizes.minimumTouchTarget, width: mobileTheme.sizes.minimumTouchTarget, justifyContent: 'center', alignItems: 'center' },
  body: { flexGrow: 1, justifyContent: 'center', padding: mobileTheme.spacing.xl },
  content: { alignItems: 'center', width: '100%', gap: mobileTheme.spacing.sm },
  pathName: { ...mobileTheme.typography.title, textAlign: 'center' },
  sessionLabel: { ...mobileTheme.typography.body, marginTop: mobileTheme.spacing.lg },
  clock: { fontSize: 64, fontWeight: '700', fontVariant: ['tabular-nums'], textAlign: 'center', width: '100%' },
  goal: { width: '100%', gap: mobileTheme.spacing.xs, marginTop: mobileTheme.spacing.lg },
  goalLabel: { ...mobileTheme.typography.body, textAlign: 'center' },
  goalTotal: { ...mobileTheme.typography.heading, textAlign: 'center' },
  track: { height: mobileTheme.sizes.progressTrack, borderRadius: mobileTheme.radii.pill, overflow: 'hidden' },
  fill: { height: '100%', borderRadius: mobileTheme.radii.pill },
  disclaimer: { ...mobileTheme.typography.caption, textAlign: 'center' },
});
