import { useCallback } from 'react';
import { useFocusEffect, useNavigation } from 'expo-router';
import {
  normalizeSocialInteractionRouteState,
  type SocialInteractionIntent,
  type SocialNavigationState,
} from './social-interaction-route-ancestry';

export function useSocialInteractionRouteAncestry(intent: SocialInteractionIntent) {
  const navigation = useNavigation();
  useFocusEffect(useCallback(() => {
    const current = navigation.getState() as SocialNavigationState;
    const normalized = normalizeSocialInteractionRouteState(current, intent);
    if (normalized !== current) navigation.reset(normalized as never);
  }, [intent.routeKey, navigation]));
}
