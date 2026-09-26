import type { Translator } from '@hourpaths/i18n';
import { ScrollView, StyleSheet, View } from 'react-native';
import type { HomePath, HomePreferences } from './home-organization';
import { SectionHeading, ThemedText as Text } from './primitives';
import { NativeButton } from './native-button';
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

function ArrangementSection({
  busy,
  collection,
  i18n,
  onMove,
  onPinChange,
  paths,
  pinned,
  reorderable,
  title,
}: {
  busy: boolean;
  collection: HomeArrangementCollection;
  i18n: Translator;
  onMove: HomeArrangementViewProps['onMove'];
  onPinChange: HomeArrangementViewProps['onPinChange'];
  paths: readonly HomePath[];
  pinned: boolean;
  reorderable: boolean;
  title: string;
}) {
  return <View style={styles.section}>
    <SectionHeading>{title}</SectionHeading>
    <View style={styles.group}>
      {paths.map((path, index) => <View key={path.id} style={[styles.row, index > 0 ? styles.separated : null]}>
        <Text style={styles.name}>{path.name}</Text>
        <View style={styles.actions}>
          {reorderable ? <>
            <NativeButton
              variant="quiet"
              accessibilityLabel={i18n.t('home.arrange.moveUp', { pathName: path.name })}
              disabled={busy || index === 0}
              onPress={() => onMove(collection, [index], index - 1)}
              label={i18n.t('home.arrange.up')}
            />
            <NativeButton
              variant="quiet"
              accessibilityLabel={i18n.t('home.arrange.moveDown', { pathName: path.name })}
              disabled={busy || index === paths.length - 1}
              onPress={() => onMove(collection, [index], index + 2)}
              label={i18n.t('home.arrange.down')}
            />
          </> : null}
          <NativeButton
            variant="quiet"
            accessibilityLabel={i18n.t(pinned ? 'home.arrange.unpin' : 'home.arrange.pin', { pathName: path.name })}
            disabled={busy}
            onPress={() => onPinChange(path.id, !pinned)}
            label={i18n.t(pinned ? 'home.arrange.unpinShort' : 'home.arrange.pinShort')}
          />
        </View>
      </View>)}
    </View>
  </View>;
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
  return <ScrollView contentContainerStyle={styles.content}>
    {pinned.length > 0 ? <ArrangementSection
      busy={busy}
      collection="pinned"
      i18n={i18n}
      onMove={onMove}
      onPinChange={onPinChange}
      paths={pinned}
      pinned
      reorderable
      title={i18n.t('home.arrange.pinnedHeading')}
    /> : null}
    <ArrangementSection
      busy={busy}
      collection="manual"
      i18n={i18n}
      onMove={onMove}
      onPinChange={onPinChange}
      paths={manual}
      pinned={false}
      reorderable={preferences.order === 'manual'}
      title={i18n.t('home.arrange.pathsHeading')}
    />
  </ScrollView>;
}

const styles = StyleSheet.create({
  actions: {
    alignItems: 'center',
    flexDirection: 'row',
    flexWrap: 'wrap',
  },
  content: {
    gap: mobileTheme.spacing.lg,
    padding: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.xxl,
  },
  group: {
    backgroundColor: mobileTheme.colors.surface,
    borderRadius: mobileTheme.radii.md,
    overflow: 'hidden',
  },
  name: {
    ...mobileTheme.typography.body,
  },
  row: {
    alignItems: 'stretch',
    gap: mobileTheme.spacing.sm,
    minHeight: 56,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.xs,
  },
  section: {
    gap: mobileTheme.spacing.xs,
  },
  separated: {
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
});
