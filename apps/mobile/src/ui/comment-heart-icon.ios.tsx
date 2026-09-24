import { Host, Image as SwiftUIImage } from '@expo/ui/swift-ui';
import { StyleSheet } from 'react-native';
import { mobileTheme } from './tokens';

export function CommentHeartIcon({ selected }: { selected: boolean }) {
  return <Host style={styles.host}>
    <SwiftUIImage
      color={selected ? mobileTheme.colors.accent : mobileTheme.colors.textMuted}
      size={17}
      systemName={selected ? 'heart.fill' : 'heart'}
    />
  </Host>;
}

const styles = StyleSheet.create({ host: { height: 20, width: 20 } });
