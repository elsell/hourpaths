import { NativeHost as Host } from './native-host';
import { Button, HStack, List, Section, Spacer, Text } from '@expo/ui/swift-ui';
import {
  accessibilityLabel,
  buttonStyle,
  disabled,
  environment,
  listStyle,
  tint,
} from '@expo/ui/swift-ui/modifiers';
import type { Translator } from '@hourpaths/i18n';
import { StyleSheet, View } from 'react-native';
import type { HomePath, HomePreferences } from './home-organization';
import { mobileTheme } from './tokens';

export type HomeArrangementCollection = 'manual' | 'pinned';

export type HomeArrangementViewProps = {
  busy: boolean;
  i18n: Translator;
  onMove: (collection: HomeArrangementCollection, sourceIndices: number[], destination: number) => void;
  onPinChange: (pathID: string, pinned: boolean) => void;
  paths: readonly HomePath[];
  preferences: HomePreferences;
};

function arrangedPaths(paths: readonly HomePath[], ids: readonly string[], include: (pathID: string) => boolean): HomePath[] {
  const index = new Map(ids.map((id, position) => [id, position]));
  return paths
    .filter(({ id }) => include(id))
    .map((path, sourceIndex) => ({ path, sourceIndex }))
    .sort((left, right) => {
      const leftIndex = index.get(left.path.id);
      const rightIndex = index.get(right.path.id);
      if (leftIndex !== undefined || rightIndex !== undefined) {
        if (leftIndex === undefined) return 1;
        if (rightIndex === undefined) return -1;
        return leftIndex - rightIndex;
      }
      return left.sourceIndex - right.sourceIndex;
    })
    .map(({ path }) => path);
}

function ArrangementRow({
  busy,
  i18n,
  onPinChange,
  path,
  pinned,
}: {
  busy: boolean;
  i18n: Translator;
  onPinChange: (pathID: string, pinned: boolean) => void;
  path: HomePath;
  pinned: boolean;
}) {
  const actionLabel = i18n.t(pinned ? 'home.arrange.unpin' : 'home.arrange.pin', { pathName: path.name });
  return <HStack alignment="center" spacing={12}>
    <Text>{path.name}</Text>
    <Spacer />
    <Button
      label={actionLabel}
      modifiers={[
        accessibilityLabel(actionLabel),
        buttonStyle('borderless'),
        disabled(busy),
        tint(mobileTheme.colors.accent),
      ]}
      onPress={() => onPinChange(path.id, !pinned)}
      systemImage={pinned ? 'pin.slash' : 'pin'}
    />
  </HStack>;
}

export function HomeArrangementView({
  busy,
  i18n,
  onMove,
  onPinChange,
  paths,
  preferences,
}: HomeArrangementViewProps) {
  const pinnedIDs = new Set(preferences.pinnedPathIDs);
  const pinned = arrangedPaths(paths, preferences.pinnedPathIDs, (id) => pinnedIDs.has(id));
  const manual = arrangedPaths(paths, preferences.manualPathIDs, (id) => !pinnedIDs.has(id));

  return <View style={styles.container}>
    <Host style={styles.host}>
      <List modifiers={[
        disabled(busy),
        environment('colorScheme', 'dark'),
        environment('editMode', 'active'),
        listStyle('insetGrouped'),
        tint(mobileTheme.colors.accent),
      ]}>
        {pinned.length > 0 ? <Section title={i18n.t('home.arrange.pinnedHeading')}>
          <List.ForEach onMove={(source, destination) => onMove('pinned', source, destination)}>
            {pinned.map((path) => <ArrangementRow
              busy={busy}
              i18n={i18n}
              key={path.id}
              onPinChange={onPinChange}
              path={path}
              pinned
            />)}
          </List.ForEach>
        </Section> : null}
        <Section title={i18n.t('home.arrange.pathsHeading')}>
          <List.ForEach
            onMove={preferences.order === 'manual'
              ? (source, destination) => onMove('manual', source, destination)
              : undefined}
          >
            {manual.map((path) => <ArrangementRow
              busy={busy}
              i18n={i18n}
              key={path.id}
              onPinChange={onPinChange}
              path={path}
              pinned={false}
            />)}
          </List.ForEach>
        </Section>
      </List>
    </Host>
  </View>;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  host: {
    flex: 1,
  },
});
