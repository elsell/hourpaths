import { getLocales } from 'expo-localization';
import { Stack } from 'expo-router';
import { createDeviceTranslator } from '../../../src/i18n';
import { nativeStackOptions } from '../../../src/ui/navigation-theme';

const i18n = createDeviceTranslator(getLocales);

export default function FollowingLayout() {
  return <Stack
    screenOptions={nativeStackOptions}
  >
    <Stack.Screen name="index" options={{ title: i18n.t('social.feedHeading') }} />
    <Stack.Screen name="people" options={{ headerLargeTitle: false, title: i18n.t('social.people') }} />
    <Stack.Screen name="activity/[pathID]/[activityID]" options={{ headerLargeTitle: false, title: i18n.t('pathDetails.activityHeading') }} />
    <Stack.Screen name="comments/[eventID]" options={{ headerLargeTitle: false, title: i18n.t('social.commentsHeading') }} />
    <Stack.Screen name="comments/[eventID]/hearts/[commentID]" options={{ headerLargeTitle: false, title: i18n.t('social.commentHeartRosterHeading') }} />
  </Stack>;
}
