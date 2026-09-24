import { useFocusEffect, useNavigation } from 'expo-router';
import { useCallback } from 'react';
import { normalizePathRouteState, type PathNavigationState } from './path-route-ancestry';
import type { PathRouteIntent } from './path-route-recovery';

export function usePathRouteAncestry(intent: PathRouteIntent) {
  const navigation = useNavigation();
  useFocusEffect(useCallback(() => {
    const current = navigation.getState() as PathNavigationState;
    const normalized = normalizePathRouteState(current, intent);
    if (normalized !== current) navigation.reset(normalized as never);
  }, [intent.routeKey, navigation]));
}
