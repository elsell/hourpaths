import { Stack } from 'expo-router';

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
  return <Stack.Toolbar placement="right">
    <Stack.Toolbar.Button
      accessibilityLabel={peopleAccessibilityLabel}
      icon="person.2"
      onPress={onOpenPeople}
    />
    <Stack.Toolbar.Button
      accessibilityLabel={followRequestsAccessibilityLabel}
      icon="person.2.badge.plus"
      onPress={onOpenFollowRequests}
    />
  </Stack.Toolbar>;
}
