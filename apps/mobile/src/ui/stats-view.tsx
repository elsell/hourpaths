import { useEffect, useState } from 'react';
import { Pressable, RefreshControl, ScrollView, StyleSheet, Text, View } from 'react-native';
import { statsRanges, statsRangeKeys, statsDuration, statsPeriodLabel, statsDateLabel, statsPathTone, statsCalendarGroups, type StatsState, type StatsSelection, type PathAppearance, type StatsCalendarUnit } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { NativeSegmentedControl } from './native-segmented-control';
import { NativeButton } from './native-button';
import { PlatformSymbol } from './platform-symbol';
import { SectionHeading, StatusBanner, Surface } from './primitives';
import { StatsBars, StatsCalendar, StatsDonut, StatsContributionGrid } from './stats-charts';
import { mobileTheme } from './tokens';

export function StatsView({ state, i18n, onSelect, onRefresh, appearance }: { state: StatsState; i18n: Translator; onSelect: (selection: StatsSelection) => void; onRefresh: () => void; appearance?: (id: string) => PathAppearance }) {
  const [pathsExpanded, setPathsExpanded] = useState(false);
  const [calendarUnit, setCalendarUnit] = useState<StatsCalendarUnit>('day');
  const { data, selection } = state;
  useEffect(() => { setCalendarUnit('day'); }, [selection.range]);
  const filterLabel = selection.pathIds.length ? i18n.t('stats.selectedPaths', { count: selection.pathIds.length }) : i18n.t('stats.allPaths');
  const chartValue = (key: string, seconds: number, unit: string) => ({ key, seconds, label: statsDateLabel(key, unit, i18n), color: mobileTheme.colors.accent });
  const selectPath = (id: string) => onSelect({ ...selection, pathIds: selection.pathIds.includes(id) ? selection.pathIds.filter(candidate => candidate !== id) : [...selection.pathIds, id] });
  return <ScrollView contentInsetAdjustmentBehavior="automatic" contentContainerStyle={styles.content} refreshControl={<RefreshControl refreshing={state.refreshing} onRefresh={onRefresh} tintColor={mobileTheme.colors.accent} />}>
    <Text style={styles.eyebrow}>{i18n.t('stats.personalOnly')}</Text>
    <NativeSegmentedControl value={selection.range} segments={statsRanges.map(value => ({ value, label: i18n.t(statsRangeKeys[value]) }))} onChange={range => onSelect({ ...selection, range, anchor: undefined })} />
    <Pressable accessibilityRole="button" accessibilityState={{ expanded: pathsExpanded }} onPress={() => setPathsExpanded(!pathsExpanded)} style={styles.filter}>
      <PlatformSymbol systemName="line.3.horizontal.decrease.circle" color={mobileTheme.colors.accent} size={22} />
      <Text style={styles.filterLabel}>{filterLabel}</Text>
      <View style={{ transform: [{ rotate: pathsExpanded ? '90deg' : '0deg' }] }}><PlatformSymbol systemName="chevron.right" size={16} /></View>
    </Pressable>
    {pathsExpanded ? <Surface>
      <PathChoice label={i18n.t('stats.allPaths')} checked={!selection.pathIds.length} onPress={() => onSelect({ ...selection, pathIds: [] })} />
      {(state.paths ?? data?.availablePaths ?? []).map(path => <PathChoice key={path.id} label={path.name} detail={path.archived ? i18n.t('stats.archived') : undefined} color={statsPathTone(path.id, appearance).background} checked={selection.pathIds.includes(path.id)} onPress={() => selectPath(path.id)} />)}
    </Surface> : null}
    {data ? <View style={styles.period}>
      {data.previousAnchor ? <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('stats.previous')} onPress={() => onSelect({ ...selection, anchor: data.previousAnchor })} style={styles.arrow}><View style={styles.back}><PlatformSymbol systemName="chevron.right" color={mobileTheme.colors.accent} size={20} /></View></Pressable> : <View style={styles.arrow} />}
      <Text style={styles.periodLabel}>{statsPeriodLabel(data, i18n)}</Text>
      {data.nextAnchor ? <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('stats.next')} onPress={() => onSelect({ ...selection, anchor: data.nextAnchor })} style={styles.arrow}><PlatformSymbol systemName="chevron.right" color={mobileTheme.colors.accent} size={20} /></Pressable> : <View style={styles.arrow} />}
    </View> : null}
    {selection.anchor ? <NativeButton variant="quiet" label={i18n.t('stats.current')} onPress={() => onSelect({ ...selection, anchor: undefined })} /> : null}
    {state.status === 'loading' || state.status === 'idle' ? <StatusBanner text={i18n.t('common.loading')} /> : null}
    {state.status === 'error' ? <StatusBanner text={i18n.t('stats.unavailable')} tone="error" actionLabel={i18n.t('common.retry')} onAction={onRefresh} /> : null}
    {data && data.totalSeconds === 0 ? <Surface>
      <Text style={styles.zero}>{statsDuration(0, i18n)}</Text>
      <SectionHeading>{i18n.t('stats.emptyTitle')}</SectionHeading>
      <Text style={styles.description}>{i18n.t('stats.emptyDescription')}</Text>
    </Surface> : data ? <>
      <Surface>
        <SectionHeading>{i18n.t('stats.distribution')}</SectionHeading>
        <StatsDonut total={data.totalSeconds} values={data.distribution.map(path => ({ key: path.pathId, label: path.name, seconds: path.seconds, color: statsPathTone(path.pathId, appearance).background }))} i18n={i18n} />
      </Surface>
      <Surface>
        <SectionHeading>{i18n.t('stats.activity')}</SectionHeading>
        <StatsBars key={JSON.stringify(selection)} values={data.buckets.map(bucket => chartValue(bucket.key, bucket.seconds, data.bucketUnit))} i18n={i18n} />
      </Surface>
    </> : null}
    {data ? <>
      <Surface>
        <SectionHeading>{i18n.t('stats.calendar')}</SectionHeading>
        <NativeSegmentedControl value={calendarUnit} segments={(['day', 'week', 'month', 'year'] as const).map(value => ({ value, label: i18n.t(calendarKeys[value]) }))} onChange={setCalendarUnit} />
        {calendarUnit === 'day' ? <StatsContributionGrid key={JSON.stringify(selection)} days={data.calendar} range={{ startDate: data.startDate, endDate: data.endDate }} weekStartsOn={data.weekStartsOn} i18n={i18n} /> : <StatsCalendar values={statsCalendarGroups(data.calendar, calendarUnit, data.weekStartsOn).map(bucket => chartValue(bucket.key, bucket.seconds, calendarUnit))} i18n={i18n} />}
      </Surface>
    </> : null}
  </ScrollView>;
}
const calendarKeys = { day: 'stats.calendar.day', week: 'stats.calendar.week', month: 'stats.calendar.month', year: 'stats.calendar.year' } as const;

function PathChoice({ label, detail, color, checked, onPress }: { label: string; detail?: string; color?: string; checked: boolean; onPress: () => void }) {
  return <Pressable accessibilityRole="checkbox" accessibilityState={{ checked }} onPress={onPress} style={styles.choice}>
    {color ? <View style={[styles.swatch, { backgroundColor: color }]} /> : null}
    <View style={styles.choiceText}><Text style={styles.filterLabel}>{label}</Text>{detail ? <Text style={styles.eyebrow}>{detail}</Text> : null}</View>
    <View style={[styles.checkbox, checked && styles.checked]}>{checked ? <PlatformSymbol systemName="checkmark" color={mobileTheme.colors.accentText} size={16} /> : null}</View>
  </Pressable>;
}
const styles = StyleSheet.create({
  content: { padding: mobileTheme.spacing.md, paddingBottom: mobileTheme.spacing.xxl, gap: mobileTheme.spacing.md },
  eyebrow: { ...mobileTheme.typography.caption, color: mobileTheme.colors.textMuted },
  filter: { flexDirection: 'row', alignItems: 'center', minHeight: 48, paddingHorizontal: mobileTheme.spacing.md, gap: mobileTheme.spacing.sm, backgroundColor: mobileTheme.colors.surface, borderRadius: mobileTheme.radii.md },
  filterLabel: { ...mobileTheme.typography.body, color: mobileTheme.colors.text, flex: 1 },
  choice: { minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.sm },
  choiceText: { flex: 1, gap: 2 },
  swatch: { width: 12, height: 12, borderRadius: 6 },
  checkbox: { width: 24, height: 24, borderRadius: 7, borderWidth: 1, borderColor: mobileTheme.colors.border, alignItems: 'center', justifyContent: 'center' },
  checked: { backgroundColor: mobileTheme.colors.accent, borderColor: mobileTheme.colors.accent },
  period: { flexDirection: 'row', alignItems: 'center' },
  arrow: { width: 48, height: 48, justifyContent: 'center', alignItems: 'center' },
  back: { transform: [{ rotate: '180deg' }] },
  periodLabel: { ...mobileTheme.typography.body, fontWeight: '600', color: mobileTheme.colors.text, flex: 1, textAlign: 'center' },
  zero: { ...mobileTheme.typography.title, color: mobileTheme.colors.text },
  description: { ...mobileTheme.typography.body, color: mobileTheme.colors.textMuted },
});
