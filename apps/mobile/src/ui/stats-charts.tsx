import { useRef, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import type { Translator } from '@hourpaths/i18n';
import { statsDuration, statsDateLabel, statsContributionWeeks, statsContributionLevel, statsWeekdayLabel, statsContributionMonthLabel } from '@hourpaths/client-core';
import { mobileTheme } from './tokens';

export type StatsChartValue = { key: string; label: string; seconds: number; color: string };

export function StatsDonut({ values, total, i18n }: { values: readonly StatsChartValue[]; total: number; i18n: Translator }) {
  let cumulative = 0;
  const bands = values.map(value => ({ ...value, end: cumulative += value.seconds / total }));
  return <View style={styles.distribution}>
    <View style={styles.donut} accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      {Array.from({ length: 180 }, (_, index) => {
        const angle = index * 2;
        const rotation = `${angle}deg` as `${number}deg`;
        const color = bands.find(band => band.end > index / 180)?.color ?? mobileTheme.colors.surfaceRaised;
        return <View key={index} style={[styles.ray, { backgroundColor: color, transform: [{ rotate: rotation }, { translateY: -46 }] }]} />;
      })}
      <View style={styles.hole}>
        <Text style={styles.total} adjustsFontSizeToFit numberOfLines={1}>{statsDuration(total, i18n)}</Text>
        <Text style={styles.muted}>{i18n.t('stats.total')}</Text>
      </View>
    </View>
    <View style={styles.legend}>
      {values.map(value => <View key={value.key} accessible accessibilityLabel={i18n.t('stats.chartValue', { label: value.label, duration: i18n.t('duration.compactSeconds', { seconds: i18n.number(value.seconds) }) })} style={styles.legendRow}>
        <View style={[styles.swatch, { backgroundColor: value.color }]} />
        <Text style={styles.name}>{value.label}</Text>
        <Text style={styles.duration}>{statsDuration(value.seconds, i18n)}</Text>
      </View>)}
    </View>
  </View>;
}

export function StatsBars({ values, i18n }: { values: readonly StatsChartValue[]; i18n: Translator }) {
  const maximum = Math.max(1, ...values.map(value => value.seconds));
  const scroll = useRef<ScrollView>(null);
  const positioned = useRef(false);
  return <ScrollView ref={scroll} horizontal showsHorizontalScrollIndicator onContentSizeChange={() => { if (!positioned.current) { scroll.current?.scrollToEnd({ animated: false }); positioned.current = true; } }} contentContainerStyle={styles.bars}>
    {values.map(value => <View key={value.key} accessible accessibilityLabel={i18n.t('stats.chartValue', { label: value.label, duration: i18n.t('duration.compactSeconds', { seconds: i18n.number(value.seconds) }) })} style={styles.barColumn}>
      <Text style={styles.barValue}>{statsDuration(value.seconds, i18n)}</Text>
      <View style={styles.barTrack}>
        <View style={[styles.bar, { height: value.seconds > 0 ? Math.max(3, value.seconds / maximum * 108) : 0, backgroundColor: value.color }]} />
      </View>
      <Text style={styles.barLabel}>{value.label}</Text>
    </View>)}
  </ScrollView>;
}

export function StatsCalendar({ values, i18n }: { values: readonly StatsChartValue[]; i18n: Translator }) {
  const maximum = Math.max(1, ...values.map(value => value.seconds));
  return <View style={styles.calendar}>
    {values.map(value => <View key={value.key} accessible accessibilityLabel={i18n.t('stats.chartValue', { label: value.label, duration: i18n.t('duration.compactSeconds', { seconds: i18n.number(value.seconds) }) })} style={styles.calendarCell}>
      <View style={styles.calendarTrack}>
        <View style={[StyleSheet.absoluteFill, { backgroundColor: value.color, opacity: value.seconds ? 0.2 + 0.8 * value.seconds / maximum : 0 }]} />
      </View>
      <Text style={styles.calendarLabel}>{value.label}</Text>
      <Text style={styles.calendarDuration}>{statsDuration(value.seconds, i18n)}</Text>
    </View>)}
  </View>;
}

export function StatsContributionGrid({ days, weekStartsOn, range, i18n }: { range: { startDate: string; endDate: string }; days: readonly { date: string; seconds: number }[]; weekStartsOn: number; i18n: Translator }) {
  const weeks = statsContributionWeeks(days, weekStartsOn, range);
  const scroll = useRef<ScrollView>(null);
  const positioned = useRef(false);
  const maximum = Math.max(1, ...days.map(day => day.seconds));
  const [selected, setSelected] = useState<string>();
  const selectedDay = weeks.flat().find(day => day?.date === selected);
  const label = (day: { date: string; seconds: number }) => i18n.t('stats.chartValue', { label: statsDateLabel(day.date, 'day', i18n), duration: i18n.t('duration.compactSeconds', { seconds: i18n.number(day.seconds) }) });
  return <View style={styles.contribution}>
    <View style={styles.gridBody}><View style={styles.weekdays}>{Array.from({ length: 7 }, (_, index) => <Text key={index} style={styles.weekday}>{statsWeekdayLabel(index, weekStartsOn, i18n)}</Text>)}</View>
    <ScrollView ref={scroll} horizontal showsHorizontalScrollIndicator contentContainerStyle={styles.weeks}
      onContentSizeChange={() => { if (!positioned.current) { scroll.current?.scrollToEnd({ animated: false }); positioned.current = true; } }}>
      {weeks.map((week, index) => <View key={index} style={styles.week}>
        <Text numberOfLines={1} style={styles.month}>{statsContributionMonthLabel(week, index, i18n)}</Text>
        {week.map((day, weekday) => day ? <Pressable key={day.date} accessibilityRole="button" accessibilityLabel={label(day)} accessibilityState={{ selected: selected === day.date }} onPress={() => setSelected(day.date)} style={[styles.day, { backgroundColor: contributionColors[statsContributionLevel(day.seconds, maximum)] }, selected === day.date && styles.selectedDay]} /> : <View key={weekday} style={styles.dayPlaceholder} />)}
      </View>)}
    </ScrollView></View>
    <View style={styles.gridLegend}><Text style={styles.muted}>{i18n.t('stats.contribution.less')}</Text>{contributionColors.map(color => <View key={color} style={[styles.legendDay, { backgroundColor: color }]} />)}<Text style={styles.muted}>{i18n.t('stats.contribution.more')}</Text></View>
    {selectedDay ? <Text accessibilityLiveRegion="polite" style={styles.muted}>{label(selectedDay)}</Text> : null}
  </View>;
}
const contributionColors = [mobileTheme.colors.surfaceRaised, '#514A26', '#807137', '#B49B40', mobileTheme.colors.accent];

const styles = StyleSheet.create({
  gridBody: { flexDirection: 'row', gap: 5 },
  weekdays: { gap: 4, paddingTop: 25 },
  weekday: { height: 22, lineHeight: 22, fontSize: 10, color: mobileTheme.colors.textMuted },
  month: { height: 18, fontSize: 9, color: mobileTheme.colors.textMuted, width: 22 },
  contribution: { gap: mobileTheme.spacing.sm },
  weeks: { gap: 4, paddingVertical: 3 },
  week: { gap: 4 },
  day: { width: 22, height: 22, borderRadius: 4, borderWidth: 1, borderColor: mobileTheme.colors.border },
  dayPlaceholder: { width: 22, height: 22 },
  selectedDay: { borderWidth: 2, borderColor: mobileTheme.colors.text },
  gridLegend: { flexDirection: 'row', alignItems: 'center', justifyContent: 'flex-end', gap: 4 },
  legendDay: { width: 12, height: 12, borderRadius: 3 },
  distribution: { gap: mobileTheme.spacing.lg },
  donut: { width: 184, height: 184, borderRadius: 92, overflow: 'hidden', alignSelf: 'center', alignItems: 'center', justifyContent: 'center' },
  ray: { position: 'absolute', width: 4, height: 92, left: 90, top: 46 },
  hole: { width: 138, height: 138, borderRadius: 69, backgroundColor: mobileTheme.colors.surface, alignItems: 'center', justifyContent: 'center', padding: 12, gap: 4 },
  total: { ...mobileTheme.typography.heading, color: mobileTheme.colors.text, fontVariant: ['tabular-nums'] },
  muted: { ...mobileTheme.typography.caption, color: mobileTheme.colors.textMuted, textAlign: 'center' },
  legend: { gap: mobileTheme.spacing.sm },
  legendRow: { flexDirection: 'row', alignItems: 'center', gap: mobileTheme.spacing.xs },
  swatch: { width: 10, height: 10, borderRadius: 5 },
  name: { ...mobileTheme.typography.body, color: mobileTheme.colors.text, flex: 1 },
  duration: { ...mobileTheme.typography.body, color: mobileTheme.colors.text, fontVariant: ['tabular-nums'] },
  bars: { gap: mobileTheme.spacing.xs, paddingBottom: mobileTheme.spacing.sm, flexGrow: 1 },
  barColumn: { alignItems: 'center', width: 55, gap: 6, flexGrow: 1 },
  barTrack: { height: 108, width: 24, justifyContent: 'flex-end', borderBottomWidth: 1, borderBottomColor: mobileTheme.colors.separator },
  bar: { width: 24, borderTopLeftRadius: 5, borderTopRightRadius: 5 },
  barValue: { ...mobileTheme.typography.caption, color: mobileTheme.colors.textMuted, textAlign: 'center' },
  barLabel: { ...mobileTheme.typography.caption, color: mobileTheme.colors.textMuted, textAlign: 'center' },
  calendar: { flexDirection: 'row', flexWrap: 'wrap', gap: mobileTheme.spacing.xs },
  calendarCell: { width: 64, flexGrow: 1, maxWidth: 90, gap: 3 },
  calendarTrack: { height: 28, borderRadius: 6, overflow: 'hidden', backgroundColor: mobileTheme.colors.surfaceRaised },
  calendarLabel: { ...mobileTheme.typography.caption, color: mobileTheme.colors.textMuted },
  calendarDuration: { fontSize: 11, color: mobileTheme.colors.text, fontVariant: ['tabular-nums'] },
});
