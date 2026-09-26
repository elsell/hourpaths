import { getLocales } from 'expo-localization';
import { NativeTabs } from 'expo-router/unstable-native-tabs';
import { mobileTheme } from '../../src/ui/tokens';
import { createDeviceTranslator } from '../../src/i18n';

const i18n = createDeviceTranslator(getLocales);

export default function TabLayout() {
  return <NativeTabs
    tintColor={mobileTheme.colors.accent}
    minimizeBehavior="onScrollDown"
  >
    <NativeTabs.Trigger name="home">
      <NativeTabs.Trigger.Label>{i18n.t('home.heading')}</NativeTabs.Trigger.Label>
      <NativeTabs.Trigger.Icon sf={{ default: 'house', selected: 'house.fill' }} md="home" />
    </NativeTabs.Trigger>
    <NativeTabs.Trigger name="following">
      <NativeTabs.Trigger.Label>{i18n.t('social.following')}</NativeTabs.Trigger.Label>
      <NativeTabs.Trigger.Icon sf={{ default: 'person.2', selected: 'person.2.fill' }} md="group" />
    </NativeTabs.Trigger>
  </NativeTabs>;
}
