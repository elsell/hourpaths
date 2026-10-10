import type { NotificationHistoryItem } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { NativeSheet, StatusBanner } from './primitives';
import { SettingsNavigationRow, SettingsSection } from './settings-list';

type Reminder = Extract<NotificationHistoryItem, { type: 'goal_practice_reminder' }>;

export function GoalReminderSheet({ i18n, notificationId, load, onClose, onOpenPath }: {
  i18n: Translator;
  notificationId: string;
  load: (id: string) => Promise<Reminder>;
  onClose: () => void;
  onOpenPath: (id: string) => void;
}) {
  const [reminder, setReminder] = useState<Reminder | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let active = true;
    setReminder(null);
    setFailed(false);
    void load(notificationId).then(value => { if (active) setReminder(value); }, () => { if (active) setFailed(true); });
    return () => { active = false; };
  }, [notificationId, load]);
  return <NativeSheet visible title={i18n.t('notification.goalReminderChooserTitle')} onRequestClose={onClose}
    trailingAction={{ label: i18n.t('common.done'), onPress: onClose }}>
    {failed ? <StatusBanner tone="error" text={i18n.t('notification.error')} />
      : !reminder ? <ActivityIndicator accessibilityLabel={i18n.t('notification.loading')} />
      : <SettingsSection>{reminder.reminder.paths.map(path => <SettingsNavigationRow key={path.id}
        label={path.name} accessibilityLabel={path.name} onPress={() => onOpenPath(path.id)} />)}</SettingsSection>}
  </NativeSheet>;
}
