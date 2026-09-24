import type { Translator } from '@hourpaths/i18n';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativePrimaryButton } from './native-primary-button';
import { ThemedText as Text } from './primitives';
import { SocialProfileAvatar } from './social-profile-avatar';
import type { SocialProfileDetailState } from './social-profile-route-presentation';
import { mobileTheme } from './tokens';

export function SocialProfileDetailView({
  i18n,
  blockingStatus,
  onRetry,
  onRelationshipAction,
  state,
}: {
  i18n: Translator;
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
      <Pressable accessibilityRole="button" onPress={onRetry} style={styles.retry}>
        <Text style={styles.retryLabel}>{i18n.t('common.retry')}</Text>
      </Pressable>
    </View>;
  }

  return <View style={styles.content}>
    <View style={styles.identity}>
      <SocialProfileAvatar
        accessibilityLabel={i18n.t('social.neutralAvatarLabel')}
        size={88}
      />
      <View style={styles.names}>
        <Text accessibilityRole="header" style={styles.displayName}>{profile.displayName}</Text>
        <Text style={styles.username}>@{profile.username}</Text>
      </View>
      {profile.description ? <Text style={styles.description}>{profile.description}</Text> : null}
      {profile.relationship !== 'self' ? <NativePrimaryButton
        disabled={state.mutating}
        label={i18n.t(profile.relationship === 'none'
          ? 'social.follow'
          : profile.relationship === 'requested'
            ? 'social.requested'
            : 'social.followingAction')}
        onPress={() => onRelationshipAction(profile.relationship === 'none'
          ? 'follow'
          : profile.relationship === 'requested'
            ? 'cancel-request'
            : 'unfollow')}
        systemImage={profile.relationship === 'none' ? 'person.badge.plus' : undefined}
        variant={profile.relationship === 'none' ? 'prominent' : 'plain'}
      /> : null}
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
    <View style={styles.counts}>
      <View accessible accessibilityLabel={`${i18n.number(profile.followerCount)} ${i18n.t('social.profileFollowers')}`} style={styles.count}>
        <Text style={styles.countValue}>{i18n.number(profile.followerCount)}</Text>
        <Text style={styles.countLabel}>{i18n.t('social.profileFollowers')}</Text>
      </View>
      <View accessible accessibilityLabel={`${i18n.number(profile.followingCount)} ${i18n.t('social.profileFollowing')}`} style={styles.count}>
        <Text style={styles.countValue}>{i18n.number(profile.followingCount)}</Text>
        <Text style={styles.countLabel}>{i18n.t('social.profileFollowing')}</Text>
      </View>
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
    alignItems: 'center',
    flexDirection: 'row',
    gap: mobileTheme.spacing.xs,
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  blockingStatusText: {
    color: mobileTheme.colors.textMuted,
    fontSize: 15,
    lineHeight: 20,
  },
  content: {
    gap: mobileTheme.spacing.xl,
    padding: mobileTheme.spacing.lg,
  },
  count: {
    alignItems: 'flex-start',
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: StyleSheet.hairlineWidth,
    flexBasis: 140,
    flexGrow: 1,
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
    flexWrap: 'wrap',
    gap: mobileTheme.spacing.md,
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
    textAlign: 'center',
  },
  displayName: {
    fontSize: 24,
    fontWeight: '700',
    lineHeight: 30,
    textAlign: 'center',
  },
  identity: {
    alignItems: 'center',
    gap: mobileTheme.spacing.md,
  },
  names: {
    alignItems: 'center',
    gap: mobileTheme.spacing.xxs,
  },
  mutationError: {
    color: mobileTheme.colors.error,
    fontSize: 14,
    lineHeight: 19,
    textAlign: 'center',
  },
  retry: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
  retryLabel: {
    color: mobileTheme.colors.accent,
    fontSize: 17,
    fontWeight: '600',
  },
  username: {
    color: mobileTheme.colors.textMuted,
    fontSize: 17,
    lineHeight: 22,
  },
});
