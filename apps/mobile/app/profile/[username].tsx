import * as Crypto from 'expo-crypto';
import { getLocales } from 'expo-localization';
import { router, Stack, useFocusEffect, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { AccessibilityInfo, Alert } from 'react-native';
import type { BlockReviewAcknowledgement, UserBlockingPort } from '@hourpaths/client-core';
import { createDeviceTranslator } from '../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../src/social-route-recovery';
import { NativeRouteScreen } from '../../src/ui/native-route-presentation';
import { PathHeaderMenu } from '../../src/ui/path-header-menu';
import { SocialProfileDetailView } from '../../src/ui/social-profile-detail-view';
import { canBlockCurrentSocialProfile, useSocialProfileRoutePresentation } from '../../src/ui/social-profile-route-presentation';
import { useSocialRouteRecovery } from '../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../src/ui/social-route-recovery-view';
import { useUserBlockingRoutePresentation } from '../../src/ui/user-blocking-route-presentation';

const i18n = createDeviceTranslator(getLocales);

export default function SocialProfileScreen() {
  const { username } = useLocalSearchParams<{ username: string }>();
  const social = useSocialProfileRoutePresentation();
  const presentation = useUserBlockingRoutePresentation();
  const recovery = useSocialRouteRecovery({
    kind: 'profile',
    routeKey: `social:profile:${username?.toLowerCase() ?? ''}`,
    username: username ?? '',
  });
  const [blockingStatus, setBlockingStatus] = useState<'idle' | 'reviewing' | 'blocking' | 'error'>('idle');
  const blockOperation = useRef(0);
  const blockSubmitting = useRef(false);
  const hasFocused = useRef(false);
  const requestedUsername = useRef<string | undefined>(undefined);
  const refreshRef = useRef<(() => void) | undefined>(undefined);
  refreshRef.current = social?.refreshProfile;
  useEffect(() => {
    if (!username || social || presentation || recovery) return;
    return scheduleSocialRouteBootstrap(
      { kind: 'profile', routeKey: `social:profile:${username.toLowerCase()}`, username },
      () => router.replace('/(tabs)/home'),
    );
  }, [presentation, recovery, social, username]);

  useEffect(() => {
    blockOperation.current += 1;
    blockSubmitting.current = false;
    setBlockingStatus('idle');
  }, [username]);

  useEffect(() => {
    if (!username || social?.profile.username === username || requestedUsername.current === username) return;
    requestedUsername.current = username;
    social?.loadProfile(username);
  }, [social?.loadProfile, social?.profile.username, username]);

  useFocusEffect(useCallback(() => {
    if (hasFocused.current) refreshRef.current?.();
    else hasFocused.current = true;
  }, []));

  async function completeBlock(targetUserId: string, expectedUsername: string, acknowledgement: BlockReviewAcknowledgement, idempotencyKey: string, ownedOperation: number, ownedPresentation: UserBlockingPort) {
    if (!username || username.toLowerCase() !== expectedUsername.toLowerCase() || !canBlockCurrentSocialProfile(expectedUsername) ||
      ownedOperation !== blockOperation.current || blockSubmitting.current) return;
    blockSubmitting.current = true;
    setBlockingStatus('blocking');
    try {
      const result = await ownedPresentation.blockUser(expectedUsername, idempotencyKey, acknowledgement);
      if (ownedOperation !== blockOperation.current) return;
      if (!result.blocked || result.target.userId !== targetUserId) throw new Error('mismatched_block_target');
      await AccessibilityInfo.announceForAccessibility(i18n.t('blocking.blockedSuccess', { username: expectedUsername }));
      router.replace('/(tabs)/following');
    } catch {
      if (ownedOperation === blockOperation.current) setBlockingStatus('error');
    } finally {
      if (ownedOperation === blockOperation.current) blockSubmitting.current = false;
    }
  }

  async function prepareBlock() {
    if (!username || !presentation || blockingStatus === 'reviewing' || blockingStatus === 'blocking') return;
    const ownedPresentation = presentation;
    const ownedOperation = ++blockOperation.current;
    setBlockingStatus('reviewing');
    try {
      const review = await ownedPresentation.reviewBlock(username);
      if (ownedOperation !== blockOperation.current) return;
      if (review.target.username.toLowerCase() !== username.toLowerCase()) throw new Error('mismatched_block_review');
      const idempotencyKey = Crypto.randomUUID();
      const sharedPathWarning = review.sharedPaths.length > 0
        ? [
            i18n.t('blocking.sharedPathsWarning', {
              count: review.sharedPaths.length,
              paths: new Intl.ListFormat(i18n.locale, { style: 'long', type: 'conjunction' })
                .format(review.sharedPaths.map(({ name }) => name)),
              username: review.target.username,
            }),
            i18n.t('blocking.leavePathsSeparately'),
          ].join('\n\n')
        : '';
      setBlockingStatus('idle');
      Alert.alert(
        i18n.t('blocking.confirmTitle', { username: review.target.username }),
        [i18n.t('blocking.confirmDescription'), sharedPathWarning].filter(Boolean).join('\n\n'),
        [
          { style: 'cancel', text: i18n.t('common.cancel') },
          {
            onPress: () => void completeBlock(review.target.userId, review.target.username, review.acknowledgement, idempotencyKey, ownedOperation, ownedPresentation),
            style: 'destructive',
            text: i18n.t('blocking.blockAction'),
          },
        ],
      );
    } catch {
      if (ownedOperation === blockOperation.current) setBlockingStatus('error');
    }
  }

  if (!social || !presentation) return <SocialRouteRecoveryView
    i18n={i18n}
    onGoFollowing={recovery?.onGoFollowing}
    onGoHome={recovery?.onGoHome}
    onRetry={recovery?.onRetry}
    state={recovery?.state ?? 'loading'}
  />;
  const profile = social.profile.username === username
    ? social.profile
    : { refreshing: false, status: 'loading' as const, username };
  const actionProfile = profile.status === 'ready' && profile.profile?.relationship !== 'self'
    ? profile.profile
    : undefined;
  return <>
    {actionProfile ? <PathHeaderMenu
      accessibilityLabel={i18n.t('blocking.profileActions', { username: actionProfile.username })}
      actions={[{
        destructive: true,
        disabled: blockingStatus === 'reviewing' || blockingStatus === 'blocking',
        label: i18n.t('blocking.blockActionLabel', { username: actionProfile.username }),
        onPress: () => void prepareBlock(),
        systemImage: 'hand.raised',
      }]}
    /> : <Stack.Screen options={{ headerRight: undefined }} />}
    <NativeRouteScreen
      onRefresh={social.refreshProfile}
      refreshing={profile.refreshing}
    >
      <SocialProfileDetailView
        blockingStatus={blockingStatus === 'idle' ? undefined : blockingStatus}
        i18n={i18n}
        onRetry={social.retryProfile}
        onRelationshipAction={(action) => {
          if (!username) return;
          if (action === 'unfollow') {
            const idempotencyKey = Crypto.randomUUID();
            Alert.alert(
              i18n.t('social.unfollowConfirmTitle'),
              i18n.t('social.unfollowConfirmDescription', { username }),
              [
                { style: 'cancel', text: i18n.t('common.cancel') },
                { style: 'destructive', text: i18n.t('social.unfollow'), onPress: () => social.mutateRelationship(action, username, idempotencyKey) },
              ],
            );
            return;
          }
          social.mutateRelationship(action, username, Crypto.randomUUID());
        }}
        state={profile}
      />
    </NativeRouteScreen>
  </>;
}
