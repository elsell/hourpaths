import { getLocales } from 'expo-localization';
import { Stack } from 'expo-router';
import { createDeviceTranslator } from '../src/i18n';
import { NativeHeaderButton } from '../src/ui/native-header-button';
import { useNotificationRoutePresentation } from '../src/ui/notification-route-presentation';
import { mobileTheme } from '../src/ui/tokens';

const i18n = createDeviceTranslator(getLocales);

export default function RootLayout() {
  const notificationPresentation = useNotificationRoutePresentation();
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
    }}
  >
    <Stack.Screen name="index" options={{ headerShown: false }} />
    <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
    <Stack.Screen name="notifications" options={{
      headerLargeTitle: false,
      headerRight: notificationPresentation
        ? () => <NativeHeaderButton
            accessibilityLabel={i18n.t('notification.markAllRead')}
            disabled={notificationPresentation.busy || !notificationPresentation.canMarkAllRead}
            label={i18n.t('notification.markAllRead')}
            onPress={notificationPresentation.markAllRead}
            systemImage="checkmark.circle"
          />
        : undefined,
      title: i18n.t('notification.heading'),
    }} />
    <Stack.Screen name="invitations" options={{ headerLargeTitle: false, title: i18n.t('pathInvitation.pendingHeading') }} />
    <Stack.Screen name="profile/[username]" options={{ headerLargeTitle: false, title: i18n.t('social.profileHeading') }} />
    <Stack.Screen name="follow-requests" options={{ headerLargeTitle: false, title: i18n.t('social.followRequests') }} />
    <Stack.Screen name="settings/index" options={{ headerLargeTitle: false, title: i18n.t('settings.heading') }} />
    <Stack.Screen name="settings/account" options={{ headerLargeTitle: false, title: i18n.t('settings.account') }} />
    <Stack.Screen name="settings/notifications" options={{ headerLargeTitle: false, title: i18n.t('notification.settings.heading') }} />
    <Stack.Screen name="settings/time-zone" options={{ headerLargeTitle: false, title: i18n.t('settings.timeZone.heading') }} />
    <Stack.Screen name="settings/interactions" options={{ headerLargeTitle: false, title: i18n.t('settings.interactions.heading') }} />
    <Stack.Screen name="settings/blocked-accounts" options={{ headerLargeTitle: false, title: i18n.t('blocking.settingsHeading') }} />
    <Stack.Screen name="path/[pathID]" options={{ headerLargeTitle: false }} />
    <Stack.Screen name="path/[pathID]/members/index" options={{ headerLargeTitle: false, title: i18n.t('pathMembers.heading') }} />
    <Stack.Screen name="path/[pathID]/members/[userID]" options={{ headerLargeTitle: false, title: i18n.t('pathMembers.memberHeading') }} />
    <Stack.Screen name="path/[pathID]/nudge-settings" options={{ headerLargeTitle: false, title: i18n.t('nudge.audience.heading') }} />
    <Stack.Screen name="path/[pathID]/history/index" options={{ headerLargeTitle: false, title: i18n.t('pathDetails.history') }} />
    <Stack.Screen name="path/[pathID]/history/[activityID]" options={{ headerLargeTitle: false, title: i18n.t('pathDetails.activityHeading') }} />
  </Stack>;
}
