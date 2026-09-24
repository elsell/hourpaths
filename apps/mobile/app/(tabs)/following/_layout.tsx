import { getLocales } from 'expo-localization';
import { Stack } from 'expo-router';
import { createDeviceTranslator } from '../../../src/i18n';
import { mobileTheme } from '../../../src/ui/tokens';

const i18n = createDeviceTranslator(getLocales);

export default function FollowingLayout() {
  return <Stack
    screenOptions={{
      animation: 'default',
      contentStyle: { backgroundColor: mobileTheme.colors.background },
      gestureEnabled: true,
      headerBackButtonDisplayMode: 'minimal',
      headerLargeTitle: true,
      headerShadowVisible: false,
      headerTintColor: mobileTheme.colors.accent,
      headerTitleStyle: { color: mobileTheme.colors.text },
      statusBarStyle: 'light',
    }}
  >
    <Stack.Screen name="index" options={{ title: i18n.t('social.feedHeading') }} />
    <Stack.Screen name="people" options={{ headerLargeTitle: false, title: i18n.t('social.people') }} />
    <Stack.Screen name="activity/[pathID]/[activityID]" options={{ headerLargeTitle: false, title: i18n.t('pathDetails.activityHeading') }} />
    <Stack.Screen name="comments/[eventID]" options={{ headerLargeTitle: false, title: i18n.t('social.commentsHeading') }} />
    <Stack.Screen name="comments/[eventID]/hearts/[commentID]" options={{ headerLargeTitle: false, title: i18n.t('social.commentHeartRosterHeading') }} />
  </Stack>;
}
