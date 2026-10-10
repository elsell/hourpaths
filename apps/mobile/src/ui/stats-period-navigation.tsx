import { Pressable, StyleSheet, Text, View } from 'react-native';
import { statsPeriodLabel, type StatsSelection, type StatsState } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { NativeButton } from './native-button';
import { PlatformSymbol } from './platform-symbol';
import { mobileTheme } from './tokens';

export function StatsPeriodNavigation({ state, i18n, onSelect }: { state: StatsState; i18n: Translator; onSelect: (selection: StatsSelection) => void }) {
  const { data, selection } = state;
  const navigable = selection.range !== 'past_year' && selection.range !== 'all_time';
  return <>
    {data ? <View style={styles.period}>
      {navigable && data.previousAnchor ? <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('stats.previous')} onPress={() => onSelect({ ...selection, anchor: data.previousAnchor })} style={styles.arrow}><View style={styles.back}><PlatformSymbol systemName="chevron.right" color={mobileTheme.colors.accent} size={20} /></View></Pressable> : null}
      <Text style={styles.label}>{statsPeriodLabel(data, i18n)}</Text>
      {navigable && data.nextAnchor ? <Pressable accessibilityRole="button" accessibilityLabel={i18n.t('stats.next')} onPress={() => onSelect({ ...selection, anchor: data.nextAnchor })} style={styles.arrow}><PlatformSymbol systemName="chevron.right" color={mobileTheme.colors.accent} size={20} /></Pressable> : null}
    </View> : null}
    {selection.anchor ? <NativeButton variant="quiet" label={i18n.t('stats.current')} onPress={() => onSelect({ ...selection, anchor: undefined })} /> : null}
  </>;
}
const styles = StyleSheet.create({
  period: { flexDirection: 'row', alignItems: 'center' },
  arrow: { width: 48, height: 48, justifyContent: 'center', alignItems: 'center' },
  back: { transform: [{ rotate: '180deg' }] },
  label: { ...mobileTheme.typography.body, fontWeight: '600', color: mobileTheme.colors.text, flex: 1, textAlign: 'center' },
});
