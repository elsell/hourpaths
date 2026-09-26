import { SymbolView, type AndroidSymbol } from 'expo-symbols';
import { View } from 'react-native';
import { mobileTheme } from './tokens';

// Presentation adapters translate semantic names; screens do not draw their own icons.
const materialSymbols: Readonly<Record<string, AndroidSymbol>> = {
  'archivebox': 'archive', 'archivebox.fill': 'archive',
  'arrow.clockwise': 'refresh', 'arrow.up': 'arrow_upward',
  'bell': 'notifications', 'bell.slash': 'notifications_off',
  'bubble.left': 'chat_bubble', 'bubble.left.and.bubble.right': 'forum',
  'checkmark': 'check', 'checkmark.circle': 'check_circle',
  'chevron.right': 'chevron_right', 'clock.arrow.circlepath': 'history',
  'ellipsis': 'more_horiz', 'exclamationmark.triangle': 'warning',
  'globe': 'language', 'hand.raised': 'block', 'hand.raised.fill': 'block',
  'hand.wave': 'waving_hand', 'heart': 'favorite_border', 'heart.fill': 'favorite',
  'house': 'home', 'house.fill': 'home', 'line.3.horizontal.decrease.circle': 'filter_list',
  'pencil': 'edit', 'person.2': 'group', 'person.2.fill': 'group', 'person.2.slash': 'group_off',
  'person.badge.clock': 'pending_actions', 'person.badge.plus': 'person_add',
  'person.crop.circle': 'account_circle', 'person.crop.circle.fill': 'account_circle',
  'person.crop.circle.badge.checkmark': 'verified_user',
  'person.crop.circle.badge.exclamationmark': 'person_alert',
  'person.crop.circle.badge.questionmark': 'person_search',
  'pin': 'push_pin', 'pin.slash': 'keep_off', 'play.fill': 'play_arrow', 'plus': 'add',
  'rectangle.portrait.and.arrow.right': 'logout', 'slider.horizontal.3': 'tune',
  'stop.fill': 'stop', 'trash': 'delete', 'trophy.fill': 'trophy',
  'wifi.exclamationmark': 'wifi_off', 'wifi.slash': 'wifi_off',
  'xmark': 'close', 'xmark.circle': 'cancel',
};

export function PlatformSymbol({ systemName, color = mobileTheme.colors.textMuted, size = 24 }: {
  systemName: string;
  color?: string;
  size?: number;
}) {
  const name = materialSymbols[systemName] ?? 'help_outline';
  return <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
    <SymbolView name={{ android: name, web: name }} size={size} tintColor={color} />
  </View>;
}
