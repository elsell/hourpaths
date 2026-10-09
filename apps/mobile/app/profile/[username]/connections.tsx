import { getLocales } from 'expo-localization';
import { router, Stack, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef } from 'react';
import { createDeviceTranslator } from '../../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../../src/social-route-recovery';
import { useSocialProfileRoutePresentation } from '../../../src/ui/social-profile-route-presentation';
import { ProfileConnectionsView } from '../../../src/ui/profile-connections-view';
import { NativeRouteScreen } from '../../../src/ui/native-route-presentation';
import { NativeContentUnavailable } from '../../../src/ui/native-content-unavailable';
import { NativeButton } from '../../../src/ui/native-button';
import { SocialRouteRecoveryView } from '../../../src/ui/social-route-recovery-view';
const i18n = createDeviceTranslator(getLocales);
export default function ConnectionsScreen() {
  const { username = '', direction: selected } = useLocalSearchParams<{ username: string; direction?: string }>();
  const direction = selected === 'following' ? 'following' : 'followers';
  const social = useSocialProfileRoutePresentation();
  const load = useRef(social?.loadProfile); load.current = social?.loadProfile;
  const owner = social?.viewerId;
  const profileState = useRef(social?.profile); profileState.current = social?.profile;
  useEffect(() => {
    const current = profileState.current;
    if (owner && username && !(current?.status === 'ready' && current.profile?.username.toLowerCase() === username.toLowerCase())) load.current?.(username);
  }, [owner, username]);
  useEffect(() => {
    if (social || !username) return;
    return scheduleSocialRouteBootstrap({ kind: 'profile', routeKey: `social:connections:${username}:${direction}`, username }, () => router.replace('/(tabs)/home'));
  }, [social, username, direction]);
  const profile = social?.profile.profile;
  const profileReady = social?.profile.status === 'ready' && profile?.username.toLowerCase() === username.toLowerCase();
  const loading = !social || social.profile.status === 'loading';
  return <><Stack.Screen options={{ title: i18n.t(direction === 'following' ? 'social.profileFollowing' : 'social.profileFollowers') }} />
    {social?.connections && owner && profileReady ? <ProfileConnectionsView key={JSON.stringify([owner, username, direction])} i18n={i18n} repository={social.connections} viewerId={owner} username={username} direction={direction}
      canRemove={profile?.userId === owner && profile.relationship === 'self'}
      onOpen={next => router.push({ pathname: '/profile/[username]', params: { username: next } })} onChanged={social.refreshProfile} />
      : <NativeRouteScreen>{loading ? <SocialRouteRecoveryView i18n={i18n} state="loading" /> : <><NativeContentUnavailable title={i18n.t(direction === 'followers' ? 'social.profileFollowers' : 'social.profileFollowing')} description={i18n.t('connections.loadFailed')} systemImage="person.2" /><NativeButton label={i18n.t('common.retry')} onPress={() => load.current?.(username)} variant="quiet" /></>}</NativeRouteScreen>}
  </>;
}
