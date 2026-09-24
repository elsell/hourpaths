import { useCallback } from 'react';
import { useFocusEffect, useNavigation } from 'expo-router';
import {
  normalizeSettingsJourneyRouteState,
  type SettingsJourneyNavigationState,
} from './settings-journey-route-ancestry';
import type { SettingsJourneyIntent } from './settings-journey-route-recovery';

export function useSettingsJourneyRouteAncestry(intent: SettingsJourneyIntent) {
  const navigation = useNavigation();
  useFocusEffect(useCallback(() => {
    const current = navigation.getState() as SettingsJourneyNavigationState;
    const normalized = normalizeSettingsJourneyRouteState(current, intent);
    if (normalized !== current) navigation.reset(normalized as never);
  }, [intent.routeKey, navigation]));
}
