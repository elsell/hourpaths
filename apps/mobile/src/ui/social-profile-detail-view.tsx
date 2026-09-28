import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { NativeButton } from './native-button';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativePrimaryButton } from './native-primary-button';
import { ThemedText as Text } from './primitives';
import { SegmentedAvatarRing } from './segmented-avatar-ring';
import { SocialProfileAvatar } from './social-profile-avatar';
import type { SocialProfileDetailState } from './social-profile-route-presentation';
import { mobileTheme } from './tokens';

export function SocialProfileDetailView({
  i18n,
  blockingStatus,
  onRetry,
  activePathCount = 0,
  pathCount,
  onOpenActive,
  onRelationshipAction,
  state,
}: {
  i18n: Translator;
  activePathCount?: number;
  pathCount?: number;
  onOpenActive?: () => void;
  blockingStatus?: 'reviewing' | 'blocking' | 'error';
  onRetry: () => void;
  onRelationshipAction: (action: 'follow' | 'cancel-request' | 'unfollow') => void;
  state: SocialProfileDetailState;
}) {
  if (state.status === 'loading') {
    return <View
      accessibilityLabel={i18n.t('social.searchLoading')}
      accessibilityRole="progressbar"
      style={styles.centered}
    >
      <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
    </View>;
  }

  const profile = state.profile;
  if (state.status === 'error' || !profile) {
    return <View style={styles.centered}>
      <NativeContentUnavailable
        description={i18n.t(state.errorKey ?? 'social.profileUnavailableDescription')}
        systemImage="person.crop.circle.badge.exclamationmark"
        title={i18n.t('social.profileUnavailableHeading')}
      />
      <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View>;
  }

  return <View style={styles.content}>
    <View style={styles.identity}>
      <View style={styles.identityRow}>
        <Pressable disabled={!activePathCount || !onOpenActive} onPress={onOpenActive}
          accessibilityRole={activePathCount ? 'button' : undefined}
          accessibilityLabel={activePathCount ? i18n.t('social.profileOpenActive', { name: profile.displayName, count: activePathCount }) : profile.displayName}>
          <SegmentedAvatarRing count={activePathCount} avatarSize={64}>
            <SocialProfileAvatar profilePictureURL={profile.profilePictureUrl}
              accessibilityLabel={i18n.t('social.neutralAvatarLabel')} size={64} />
          </SegmentedAvatarRing>
        </Pressable>
        <View style={styles.counts}>
          {([
            ['social.profilePaths', pathCount],
            ['social.profileFollowers', profile.followerCount],
            ['social.profileFollowing', profile.followingCount],
          ] as const).map(([label, value]) => <View key={label} accessible
            accessibilityLabel={value === undefined ? i18n.t(label) : i18n.t('social.profileCountLabel', { count: value, label: i18n.t(label) })} style={styles.count}>
            <Text style={styles.countValue}>{value === undefined ? i18n.t('social.profileCountUnavailable') : i18n.number(value)}</Text>
            <Text style={styles.countLabel}>{i18n.t(label)}</Text>
          </View>)}
        </View>
      </View>
      <View style={styles.names}>
        <Text accessibilityRole="header" style={styles.displayName}>{profile.displayName}</Text>
        <Text style={styles.username}>@{profile.username}</Text>
      </View>
      {profile.description ? <Text style={styles.description}>{profile.description}</Text> : null}
      {profile.relationship !== 'self' ? <NativePrimaryButton
        fullWidth
        busy={state.mutating}
        disabled={state.mutating}
        label={i18n.t(profile.relationship === 'none'
          ? 'social.follow'
          : profile.relationship === 'requested'
            ? 'social.requested'
            : 'social.unfollow')}
        onPress={() => onRelationshipAction(profile.relationship === 'none'
          ? 'follow'
          : profile.relationship === 'requested'
            ? 'cancel-request'
            : 'unfollow')}
        systemImage={profile.relationship === 'none' ? 'person.badge.plus' : undefined}
        variant="prominent"
      /> : null}
      {state.pathsUnavailable ? <NativeButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" /> : null}
      {state.mutationErrorKey ? <Text accessibilityRole="alert" style={styles.mutationError}>
        {i18n.t(state.mutationErrorKey)}
      </Text> : null}
      {blockingStatus === 'reviewing' || blockingStatus === 'blocking' ? <View
        accessibilityLabel={i18n.t(blockingStatus === 'blocking' ? 'blocking.blocking' : 'blocking.reviewing')}
        accessibilityRole="progressbar"
        accessibilityState={{ busy: true }}
        accessibilityValue={{ text: i18n.t(blockingStatus === 'blocking' ? 'blocking.blocking' : 'blocking.reviewing') }}
        style={styles.blockingStatus}
      >
        <ActivityIndicator color={mobileTheme.colors.accent} />
        <Text style={styles.blockingStatusText}>
          {i18n.t(blockingStatus === 'blocking' ? 'blocking.blocking' : 'blocking.reviewing')}
        </Text>
      </View> : null}
      {blockingStatus === 'error' ? <Text accessibilityRole="alert" style={styles.mutationError}>
        {i18n.t('blocking.blockUnavailable')}
      </Text> : null}
    </View>
  </View>;
}

const styles = StyleSheet.create({
  centered: {
    flex: 1,
    gap: mobileTheme.spacing.sm,
    justifyContent: 'center',
    padding: mobileTheme.spacing.md,
  },
  blockingStatus: {
    alignItems: 'flex-start',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xs,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  blockingStatusText: {
    color: mobileTheme.colors.textMuted,
    fontSize: 15,
    lineHeight: 20,
  },
  identityRow: {
    flexWrap: 'wrap',
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.md,
  },
  content: {
    gap: mobileTheme.spacing.md,
  },
  count: {
    alignItems: 'center',
    flex: 1,
    minWidth: 0,
    gap: mobileTheme.spacing.xxs,
    paddingVertical: mobileTheme.spacing.sm,
  },
  countLabel: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    fontWeight: '600',
    lineHeight: 18,
  },
  counts: {
    flexDirection: 'row',
    flexBasis: 200,
    flexGrow: 1,
    gap: mobileTheme.spacing.xs,
  },
  countValue: {
    fontSize: 20,
    fontWeight: '700',
    lineHeight: 24,
  },
  description: {
    color: mobileTheme.colors.textMuted,
    fontSize: 16,
    lineHeight: 22,
    maxWidth: 520,
    textAlign: 'left',
  },
  displayName: {
    fontSize: 24,
    fontWeight: '700',
    lineHeight: 30,
    textAlign: 'left',
  },
  identity: {
    alignItems: 'stretch',
    gap: mobileTheme.spacing.md,
  },
  names: {
    alignItems: 'flex-start',
    gap: mobileTheme.spacing.xxs,
  },
  mutationError: {
    color: mobileTheme.colors.error,
    fontSize: 14,
    lineHeight: 19,
    textAlign: 'left',
  },
  username: {
    color: mobileTheme.colors.textMuted,
    fontSize: 17,
    lineHeight: 22,
  },
});
