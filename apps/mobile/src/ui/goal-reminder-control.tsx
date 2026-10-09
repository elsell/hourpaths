import * as Crypto from 'expo-crypto';
import { useEffect, useRef, useState } from 'react';
import { View } from 'react-native';
import { createGoalReminderOperationOwner, type GoalReminderPreference, type GoalReminderSubject } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { SettingsSection, SettingsSwitchRow } from './settings-list';
import { NativeButton } from './native-button';
import { ThemedText } from './primitives';
import { useSettingsPresentation, type SettingsPresentation } from './settings-presentation';

export function GoalReminderControl({ subject, i18n }: { subject: GoalReminderSubject; i18n: Translator }) {
  const presentation = useSettingsPresentation();
  const ownerKey = [presentation?.sessionKey, subject.id].join(':');
  return presentation ? <OwnedGoalReminder key={ownerKey} subject={subject} i18n={i18n} presentation={presentation} /> : null;
}
function OwnedGoalReminder({ subject, i18n, presentation }: { subject: GoalReminderSubject; i18n: Translator; presentation: SettingsPresentation }) {
  const [value, setValue] = useState<GoalReminderPreference | null>(null);
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [owner] = useState(() => createGoalReminderOperationOwner(() => Crypto.randomUUID()));
  const epoch = useRef(0);
  async function load() {
    const generation = ++epoch.current;
    setLoading(true); setFailed(false);
    try {
      const result = await presentation.goalReminders.get(subject);
      if (generation === epoch.current && presentation.isCurrent()) setValue(result);
    } catch {
      if (generation === epoch.current && presentation.isCurrent()) setFailed(true);
    } finally {
      if (generation === epoch.current && presentation.isCurrent()) setLoading(false);
    }
  }
  useEffect(() => {
    setValue(null); setBusy(false); void load();
    return () => { epoch.current++; owner.cancel(); };
  }, [presentation, subject.id, owner]);
  async function save(enabled: boolean) {
    if (!value || busy || loading || !presentation.isCurrent()) return;
    const generation = ++epoch.current;
    setBusy(true); setFailed(false);
    const result = await owner.submit(subject, { ...value, enabled }, presentation.goalReminders.update);
    if (generation !== epoch.current || !presentation.isCurrent()) return;
    if (result.kind === 'applied') setValue(result.preference);
    else if (result.kind === 'failed') setFailed(true);
    setBusy(false);
  }
  const label = i18n.t('goalReminder.path');
  return <SettingsSection footer={i18n.t('goalReminder.channelHint')}>
    <SettingsSwitchRow label={label} accessibilityLabel={label} value={value?.enabled ?? false} valueLabel={i18n.t(loading ? 'common.loading' : value?.enabled ? 'settings.interactions.on' : 'settings.interactions.off')} disabled={loading || busy || !value || failed} onValueChange={enabled => void save(enabled)} />
    {failed ? <View><ThemedText accessibilityRole="alert">{i18n.t('goalReminder.failed')}</ThemedText><NativeButton label={i18n.t('common.retry')} onPress={() => void load()} variant="quiet" /></View> : null}
  </SettingsSection>;
}
