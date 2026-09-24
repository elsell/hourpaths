import { getLocales } from 'expo-localization';
import { NativeTabs } from 'expo-router/unstable-native-tabs';
import { createDeviceTranslator } from '../../src/i18n';

const i18n = createDeviceTranslator(getLocales);

export default function TabLayout() {
  return <NativeTabs
    minimizeBehavior="onScrollDown"
  >
    <NativeTabs.Trigger name="home">
      <NativeTabs.Trigger.Label>{i18n.t('home.heading')}</NativeTabs.Trigger.Label>
      <NativeTabs.Trigger.Icon sf={{ default: 'house', selected: 'house.fill' }} />
    </NativeTabs.Trigger>
    <NativeTabs.Trigger name="following">
      <NativeTabs.Trigger.Label>{i18n.t('social.following')}</NativeTabs.Trigger.Label>
      <NativeTabs.Trigger.Icon sf={{ default: 'person.2', selected: 'person.2.fill' }} />
    </NativeTabs.Trigger>
  </NativeTabs>;
}
