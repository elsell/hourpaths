import { useCallback } from 'react';
import { useFocusEffect, useNavigation } from 'expo-router';
import {
  normalizeNotificationJourneyRouteState,
  type NotificationJourneyNavigationState,
} from './notification-journey-route-ancestry';
import type { NotificationJourneyIntent } from './notification-journey-route-recovery';

export function useNotificationJourneyRouteAncestry(intent: NotificationJourneyIntent) {
  const navigation = useNavigation();
  useFocusEffect(useCallback(() => {
    const current = navigation.getState() as NotificationJourneyNavigationState;
    const normalized = normalizeNotificationJourneyRouteState(current, intent);
    if (normalized !== current) navigation.reset(normalized as never);
  }, [intent.routeKey, navigation]));
}
