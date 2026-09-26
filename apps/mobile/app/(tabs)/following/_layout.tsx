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
  </Stack>;
}
