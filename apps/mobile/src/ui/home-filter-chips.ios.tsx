import { Button, HStack, Host, ScrollView } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  accessibilityValue,
  buttonStyle,
  controlSize,
  environment,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import type { Translator } from '@hourpaths/i18n';
import { StyleSheet, View } from 'react-native';
import { homeFilters, type HomeFilter } from './home-organization';
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
  return <View style={styles.container}>
    <Host style={styles.host}>
      <ScrollView
        axes="horizontal"
        modifiers={[environment('colorScheme', 'dark')]}
        showsIndicators={false}
      >
        <HStack spacing={8}>
          {homeFilters.map((filter) => {
            const label = i18n.t(filterKeys[filter]);
            const selected = filter === value;
            return <Button
              key={filter}
              label={label}
              modifiers={[
                accessibilityLabel(label),
                accessibilityValue(i18n.t(selected ? 'home.filter.selected' : 'home.filter.notSelected')),
                buttonStyle(selected ? 'borderedProminent' : 'bordered'),
                controlSize('regular'),
                tint(mobileTheme.colors.accent),
              ]}
              onPress={() => onChange(filter)}
            />;
          })}
        </HStack>
      </ScrollView>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    minHeight: 44,
  },
  host: {
    minHeight: 44,
  },
});
