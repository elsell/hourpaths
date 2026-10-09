import { getLocales } from 'expo-localization';
import { router } from 'expo-router';
import { useEffect, useRef } from 'react';
import { createDeviceTranslator } from '../../src/i18n';
import { queueSettingsJourneyBootstrap, scheduleSettingsJourneyBootstrap } from '../../src/settings-journey-route-recovery';
import { settingsRouteOwnerDecision } from '../../src/settings-route-state';
import { useSettingsJourneyRouteAncestry } from '../../src/use-settings-journey-route-ancestry';
import { useSettingsPresentation } from '../../src/ui/settings-presentation';
import { useSettingsJourneyRecovery } from '../../src/ui/settings-journey-route-presentation';
import { SettingsJourneyRecoveryView } from '../../src/ui/settings-journey-recovery-view';
import { WeekStartSettingsView } from '../../src/ui/week-start-settings-view';
const i18n = createDeviceTranslator(getLocales);
const intent = { kind: 'week-start', routeKey: 'settings:week-start' } as const;
export default function WeekStartSettingsRoute() {
  const published = useSettingsPresentation(), recovery = useSettingsJourneyRecovery(intent.routeKey);
  const previous = useRef<string | undefined>(undefined);
  const incoming = published?.sessionKey ?? recovery?.sessionKey;
  const decision = settingsRouteOwnerDecision(previous.current, incoming);
  if (decision === 'claim') previous.current = incoming;
  const presentation = decision === 'replacement' ? undefined : published;
  useSettingsJourneyRouteAncestry(intent);
  useEffect(() => { if (decision === 'replacement') router.replace('/(tabs)/home'); }, [decision]);
  useEffect(() => { if (!presentation && !recovery) return scheduleSettingsJourneyBootstrap(intent, () => router.replace('/(tabs)/home'), previous.current); }, [presentation, recovery]);
  if (!presentation?.weekStart) return <SettingsJourneyRecoveryView i18n={i18n} intent={intent} state={decision === 'replacement' ? 'loading' : recovery?.state ?? 'loading'} onHome={() => router.replace('/(tabs)/home')}
    onRetry={recovery?.retry ?? (() => { queueSettingsJourneyBootstrap(intent, previous.current); router.replace('/(tabs)/home'); })} />;
  return <WeekStartSettingsView key={presentation.sessionKey} i18n={i18n} repository={presentation.weekStart} />;
}
