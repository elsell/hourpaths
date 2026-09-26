import { getLocales } from 'expo-localization';
import { router, Stack } from 'expo-router';
import { useEffect } from 'react';
import { createDeviceTranslator } from '../../src/i18n';
import { scheduleSocialRouteBootstrap } from '../../src/social-route-recovery';
import type { SocialPublicProfile } from '../../src/ui/social-profile-presentation';
import { useSocialProfileRoutePresentation } from '../../src/ui/social-profile-route-presentation';
import { SocialProfileSearchView } from '../../src/ui/social-profile-search-view';
import { useSocialRouteRecovery } from '../../src/ui/social-route-recovery-presentation';
import { SocialRouteRecoveryView } from '../../src/ui/social-route-recovery-view';
import { mobileTheme } from '../../src/ui/tokens';

const i18n = createDeviceTranslator(getLocales);

export default function PeopleScreen() {
  const presentation = useSocialProfileRoutePresentation();
  const recovery = useSocialRouteRecovery({ kind: 'people', routeKey: 'social:people' });
  useEffect(() => {
    if (presentation || recovery) return;
    return scheduleSocialRouteBootstrap(
      { kind: 'people', routeKey: 'social:people' },
      () => router.replace('/(tabs)/home'),
    );
  }, [presentation, recovery]);

  function openProfile(profile: SocialPublicProfile) {
    presentation?.loadProfile(profile.username);
    router.push({ pathname: '/profile/[username]', params: { username: profile.username } });
  }

  return <>
    <Stack.Screen options={{
      headerSearchBarOptions: {
        autoCapitalize: 'none',
        hideWhenScrolling: false,
        onChangeText: (event) => presentation?.searchQuery(event.nativeEvent.text),
        placeholder: i18n.t('social.searchPlaceholder'),
        tintColor: mobileTheme.colors.accent,
      },
    }} />
    {presentation ? <SocialProfileSearchView
      i18n={i18n}
      onLoadMore={presentation.loadMore}
      onOpen={openProfile}
      onRefresh={presentation.refreshSearch}
      onRetry={presentation.retrySearch}
      state={presentation.search}
    /> : <SocialRouteRecoveryView
      i18n={i18n}
      onGoFollowing={recovery?.onGoFollowing}
      onGoHome={recovery?.onGoHome}
      onRetry={recovery?.onRetry}
      state={recovery?.state ?? 'loading'}
    />}
  </>;
}
