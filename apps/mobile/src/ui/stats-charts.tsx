import { ScrollView, StyleSheet, Text, View } from 'react-native';
import type { Translator } from '@hourpaths/i18n';
import { statsDuration } from '@hourpaths/client-core';
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
  return <ScrollView horizontal showsHorizontalScrollIndicator contentContainerStyle={styles.bars}>
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

const styles = StyleSheet.create({
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
