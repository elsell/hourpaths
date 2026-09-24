import { Linking } from 'react-native';
import { openMobilePolicyLink } from './policy-link';

export function openNativePolicyLink(url: string, unavailable: () => void): Promise<void> {
  return openMobilePolicyLink(url, { open: Linking.openURL, unavailable });
}
