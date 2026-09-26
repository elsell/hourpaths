import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ThemedText as Text } from './primitives';
import { SocialProfileAvatar } from './social-profile-avatar';
import { profileAccessibilityLabel, type SocialPublicProfile } from './social-profile-presentation';
import type { SocialProfileSearchState } from './social-profile-route-presentation';
import { SettingsIcon } from './settings-icon';
import { mobileTheme } from './tokens';

function ProfileRow({
  i18n,
  onOpen,
  profile,
}: {
  i18n: Translator;
  onOpen: () => void;
  profile: SocialPublicProfile;
}) {
  const label = profileAccessibilityLabel(profile);
  return <Pressable
    accessibilityLabel={label}
    accessibilityRole="button"
    onPress={onOpen}
    style={({ pressed }) => [styles.row, pressed ? styles.rowPressed : null]}
  >
    <SocialProfileAvatar
      accessibilityLabel={i18n.t('social.neutralAvatarLabel')}
    />
    <View style={styles.identity}>
      <Text style={styles.displayName}>{profile.displayName}</Text>
      <Text style={styles.username}>@{profile.username}</Text>
      {profile.description ? <Text style={styles.description}>{profile.description}</Text> : null}
    </View>
    <SettingsIcon systemName="chevron.right" variant="disclosure" />
  </Pressable>;
}

function SearchState({
  i18n,
  onRetry,
  state,
}: {
  i18n: Translator;
  onRetry: () => void;
  state: SocialProfileSearchState;
}) {
  if (state.status === 'loading') {
    return <View
      accessibilityLabel={i18n.t('social.searchLoading')}
      accessibilityRole="progressbar"
      style={styles.centeredState}
    >
      <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      <Text style={styles.secondary}>{i18n.t('social.searchLoading')}</Text>
    </View>;
  }

  if (state.status === 'error') {
    return <View style={styles.centeredState}>
      <NativeContentUnavailable
        description={i18n.t(state.errorKey ?? 'social.searchUnavailableDescription')}
        systemImage="wifi.exclamationmark"
        title={i18n.t('social.searchUnavailableHeading')}
      />
      <Pressable accessibilityRole="button" onPress={onRetry} style={styles.stateAction}>
        <Text style={styles.stateActionLabel}>{i18n.t('common.retry')}</Text>
      </Pressable>
    </View>;
  }

  if (state.status === 'idle') {
    return <NativeContentUnavailable
      description={i18n.t('social.searchHint')}
      systemImage="person.2.fill"
      title={i18n.t('social.searchPlaceholder')}
    />;
  }

  return <NativeContentUnavailable
    description={i18n.t('social.searchEmptyDescription')}
    systemImage="person.crop.circle.badge.questionmark"
    title={i18n.t('social.searchEmptyHeading')}
  />;
}

export function SocialProfileSearchView({
  i18n,
  onLoadMore,
  onOpen,
  onRefresh,
  onRetry,
  state,
}: {
  i18n: Translator;
  onLoadMore: () => void;
  onOpen: (profile: SocialPublicProfile) => void;
  onRefresh: () => void;
  onRetry: () => void;
  state: SocialProfileSearchState;
}) {
  const hasResults = state.items.length > 0;
  const footer = state.status === 'error' && hasResults
    ? <View accessibilityRole="alert" style={styles.partialError}>
        <Text style={styles.errorText}>
          {i18n.t(state.errorKey ?? 'social.searchUnavailableDescription')}
        </Text>
        <Pressable accessibilityRole="button" onPress={onRetry} style={styles.stateAction}>
          <Text style={styles.stateActionLabel}>{i18n.t('common.retry')}</Text>
        </Pressable>
      </View>
    : state.nextCursor
      ? <Pressable
          accessibilityRole="button"
          accessibilityState={{ busy: state.loadingMore, disabled: state.loadingMore }}
          disabled={state.loadingMore}
          onPress={onLoadMore}
          style={styles.loadMore}
        >
          {state.loadingMore ? <ActivityIndicator color={mobileTheme.colors.accent} /> : null}
          <Text style={styles.stateActionLabel}>
            {i18n.t(state.loadingMore ? 'social.loadingMore' : 'social.loadMore')}
          </Text>
        </Pressable>
      : null;
  return <ScrollView
    alwaysBounceVertical
    automaticallyAdjustContentInsets
    contentContainerStyle={[styles.content, !hasResults ? styles.emptyContent : null]}
    contentInsetAdjustmentBehavior="automatic"
    keyboardDismissMode="on-drag"
    keyboardShouldPersistTaps="handled"
    refreshControl={<RefreshControl
      colors={[mobileTheme.colors.accent]}
      onRefresh={onRefresh}
      refreshing={state.refreshing}
      tintColor={mobileTheme.colors.accent}
    />}
    style={styles.list}
  >
    {hasResults
      ? state.items.map((profile, index) => <View key={profile.userId}>
          {index > 0 ? <View style={styles.separator} /> : null}
          <ProfileRow i18n={i18n} onOpen={() => onOpen(profile)} profile={profile} />
        </View>)
      : <SearchState i18n={i18n} onRetry={onRetry} state={state} />}
    {footer}
  </ScrollView>;
}

const styles = StyleSheet.create({
  centeredState: {
    alignItems: 'stretch',
    gap: mobileTheme.spacing.sm,
    minHeight: 260,
    justifyContent: 'center',
  },
  content: {
    paddingBottom: mobileTheme.spacing.md,
  },
  description: {
    color: mobileTheme.colors.textMuted,
    fontSize: 14,
    lineHeight: 19,
  },
  displayName: {
    fontSize: 17,
    fontWeight: '600',
    lineHeight: 22,
  },
  emptyContent: {
    flexGrow: 1,
    justifyContent: 'center',
    paddingHorizontal: mobileTheme.spacing.md,
  },
  errorText: {
    color: mobileTheme.colors.error,
    textAlign: 'center',
  },
  identity: {
    flex: 1,
    minWidth: 0,
  },
  list: {
    backgroundColor: mobileTheme.colors.background,
    flex: 1,
  },
  loadMore: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xs,
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  partialError: {
    gap: mobileTheme.spacing.xs,
    padding: mobileTheme.spacing.md,
  },
  row: {
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.xs,
  },
  rowPressed: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  secondary: {
    color: mobileTheme.colors.textMuted,
    textAlign: 'center',
  },
  separator: {
    backgroundColor: mobileTheme.colors.border,
    height: StyleSheet.hairlineWidth,
    marginLeft: 76,
  },
  stateAction: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  stateActionLabel: {
    color: mobileTheme.colors.accent,
    fontSize: 17,
    fontWeight: '600',
  },
  username: {
    color: mobileTheme.colors.textMuted,
    fontSize: 15,
    lineHeight: 20,
  },
});
