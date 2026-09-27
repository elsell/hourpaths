import { getLocales } from 'expo-localization';
import { Stack } from 'expo-router';
import { createDeviceTranslator } from '../../../src/i18n';
import { nativeStackOptions } from '../../../src/ui/navigation-theme';

const i18n = createDeviceTranslator(getLocales);

export default function StatsLayout() {
  return <Stack
    screenOptions={{ ...nativeStackOptions, headerLargeTitle: false }}
  >
    <Stack.Screen name="index" options={{ title: i18n.t('stats.title') }} />
  </Stack>;
}
