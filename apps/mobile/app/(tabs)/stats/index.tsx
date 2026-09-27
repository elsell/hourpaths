import { getLocales } from 'expo-localization';
import { router, useFocusEffect } from 'expo-router';
import { useCallback, useEffect, useRef } from 'react';
import { createDeviceTranslator } from '../../../src/i18n';
import { subscribeNativeAppActive } from '../../../src/push-notifications-native';
import { useStatsPresentation } from '../../../src/ui/stats-route-presentation';
import { StatsView } from '../../../src/ui/stats-view';

const i18n = createDeviceTranslator(getLocales);
export default function StatsScreen() {
  const presentation = useStatsPresentation();
  const latest = useRef(presentation);
  const focused = useRef(false);
  latest.current = presentation;
  useFocusEffect(useCallback(() => {
    focused.current = true;
    latest.current?.onRefresh();
    return () => { focused.current = false; };
  }, []));
  useEffect(() => subscribeNativeAppActive(() => { if (focused.current) latest.current?.onRefresh(); }), []);
  useEffect(() => {
    if (presentation) return;
    const bootstrap = setTimeout(() => { if (!latest.current) router.replace('/(tabs)/home'); }, 200);
    return () => clearTimeout(bootstrap);
  }, [Boolean(presentation)]);
  useEffect(() => { if (presentation?.state.status === 'idle') presentation.onRefresh(); }, [presentation]);
  return presentation ? <StatsView state={presentation.state} i18n={i18n} onSelect={presentation.onSelect} onRefresh={presentation.onRefresh} appearance={presentation.appearance} /> : null;
}
