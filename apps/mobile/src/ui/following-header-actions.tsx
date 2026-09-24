import { View } from 'react-native';
import { NativeHeaderButton } from './native-header-button';

export function FollowingHeaderActions({
  followRequestsAccessibilityLabel,
  followRequestsLabel,
  onOpenFollowRequests,
  onOpenPeople,
  peopleAccessibilityLabel,
}: {
  followRequestsAccessibilityLabel: string;
  followRequestsLabel: string;
  onOpenFollowRequests: () => void;
  onOpenPeople: () => void;
  peopleAccessibilityLabel: string;
}) {
  return <View style={{ alignItems: 'center', flexDirection: 'row' }}>
    <NativeHeaderButton
      accessibilityLabel={peopleAccessibilityLabel}
      label={peopleAccessibilityLabel}
      onPress={onOpenPeople}
      systemImage="person.2"
    />
    <NativeHeaderButton
      accessibilityLabel={followRequestsAccessibilityLabel}
      label={followRequestsLabel}
      onPress={onOpenFollowRequests}
      systemImage="person.badge.plus"
    />
  </View>;
}
