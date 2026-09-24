import type { Translator } from '@hourpaths/i18n';
import { Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { homeFilters, type HomeFilter } from './home-organization';
import { ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

const filterKeys = {
  all: 'home.filter.all',
  shared: 'home.filter.shared',
  solo: 'home.filter.solo',
  supporting: 'home.filter.supporting',
} as const;

export function HomeFilterChips({
  i18n,
  onChange,
  value,
}: {
  i18n: Translator;
  onChange: (filter: HomeFilter) => void;
  value: HomeFilter;
}) {
  return <ScrollView
    contentContainerStyle={styles.content}
    horizontal
    showsHorizontalScrollIndicator={false}
  >
    {homeFilters.map((filter) => {
      const selected = filter === value;
      const label = i18n.t(filterKeys[filter]);
      return <Pressable
        accessibilityLabel={label}
        accessibilityRole="button"
        accessibilityState={{ selected }}
        key={filter}
        onPress={() => onChange(filter)}
        style={({ pressed }) => [
          styles.chip,
          selected ? styles.selectedChip : null,
          pressed ? styles.pressedChip : null,
        ]}
      >
        <Text style={selected ? styles.selectedLabel : styles.label}>{label}</Text>
      </Pressable>;
    })}
  </ScrollView>;
}

const styles = StyleSheet.create({
  chip: {
    alignItems: 'center',
    borderColor: mobileTheme.colors.border,
    borderRadius: mobileTheme.radii.pill,
    borderWidth: mobileTheme.sizes.border,
    justifyContent: 'center',
    minHeight: 44,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  content: {
    gap: mobileTheme.spacing.xs,
  },
  label: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.body,
  },
  pressedChip: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  selectedChip: {
    backgroundColor: mobileTheme.colors.accent,
    borderColor: mobileTheme.colors.accent,
  },
  selectedLabel: {
    color: mobileTheme.colors.accentText,
    ...mobileTheme.typography.body,
  },
});
