import type {
  NotificationHistoryState,
  PathInvitationNotification,
} from '@hourpaths/client-core';
import { notificationPresentationMessageKey } from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import { NativeActionMenu } from './native-action-menu';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ActionButton, StatusBanner, ThemedText as Text } from './primitives';
import { SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

function NotificationRow({
  busy,
  i18n,
  item,
  onDelete,
  onOpen,
}: {
  busy: boolean;
  i18n: Translator;
  item: PathInvitationNotification;
  onDelete: () => void;
  onOpen?: () => void;
}) {
  const message = i18n.t(notificationPresentationMessageKey(item), {
    displayName: item.actor.displayName,
    pathName: 'pathName' in item ? item.pathName : '',
    username: item.actor.username,
  });
  const created = i18n.date(new Date(item.createdAt), {
    day: 'numeric',
    month: 'short',
    year: new Date(item.createdAt).getFullYear() === new Date().getFullYear() ? undefined : 'numeric',
  });
  const accessibilityLabel = i18n.t('notification.rowAccessibility', {
    date: created,
    message,
    state: i18n.t(item.read ? 'notification.read' : 'notification.unread'),
  });

  return <View style={styles.row}>
    <Pressable
      accessibilityHint={onOpen ? i18n.t('notification.openHint') : undefined}
      accessibilityLabel={accessibilityLabel}
      accessibilityRole={onOpen ? 'button' : 'text'}
      accessibilityState={{ busy, disabled: !onOpen || busy }}
      disabled={!onOpen || busy}
      onPress={onOpen}
      style={({ pressed }) => [styles.rowMain, pressed ? styles.rowPressed : null]}
    >
      <View style={styles.messageLine}>
        {!item.read ? <View style={styles.unreadDot} /> : null}
        <Text style={[styles.message, !item.read ? styles.unreadMessage : null]}>{message}</Text>
      </View>
      <Text style={styles.date}>{created}</Text>
    </Pressable>
    <NativeActionMenu
      accessibilityLabel={i18n.t('notification.actionsLabel')}
      actions={[{
        disabled: busy,
        label: i18n.t('notification.delete'),
        onPress: onDelete,
        systemImage: 'trash',
      }]}
    />
  </View>;
}

export function NotificationHistoryView({
  busy,
  canOpen,
  errorKey,
  history,
  i18n,
  onDelete,
  onLoadMore,
  onOpen,
  onRetry,
}: {
  busy: boolean;
  canOpen: (notification: PathInvitationNotification) => boolean;
  errorKey?: MessageKey;
  history: NotificationHistoryState;
  i18n: Translator;
  onDelete: (notificationID: string) => void;
  onLoadMore: () => void;
  onOpen: (notification: PathInvitationNotification) => void;
  onRetry: () => void;
}) {
  const actionable = history.items.filter((item) => item.presentation === 'actionable');
  const informational = history.items.filter((item) => item.presentation === 'informational');

  if (busy && history.items.length === 0) {
    return <View accessibilityLabel={i18n.t('notification.loading')} accessibilityRole="progressbar" style={styles.loading}>
      <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      <Text style={styles.date}>{i18n.t('notification.loading')}</Text>
    </View>;
  }

  if (errorKey && history.items.length === 0) {
    return <View style={styles.state}>
      <NativeContentUnavailable
        description={i18n.t(errorKey)}
        systemImage="wifi.exclamationmark"
        title={i18n.t('notification.unavailableHeading')}
      />
      <ActionButton label={i18n.t('common.retry')} onPress={onRetry} variant="secondary" />
    </View>;
  }

  if (history.items.length === 0) {
    return <NativeContentUnavailable
      description={i18n.t('notification.emptyDescription')}
      systemImage="bell.slash"
      title={i18n.t('notification.empty')}
    />;
  }

  return <View style={styles.sections}>
    {actionable.length > 0 ? <View style={styles.section}>
      <Text accessibilityRole="header" style={styles.sectionTitle}>{i18n.t('notification.actionableHeading')}</Text>
      <View style={styles.rows}>{actionable.map((item, index) => <View key={item.id}>
        {index > 0 ? <SettingsSeparator /> : null}
        <NotificationRow
          busy={busy}
          i18n={i18n}
          item={item}
          onDelete={() => onDelete(item.id)}
          onOpen={canOpen(item) ? () => onOpen(item) : undefined}
        />
      </View>)}</View>
      <Text style={styles.footer}>{i18n.t('notification.actionableFooter')}</Text>
    </View> : null}
    {informational.length > 0 ? <View style={styles.section}>
      <Text accessibilityRole="header" style={styles.sectionTitle}>{i18n.t('notification.informationalHeading')}</Text>
      <View style={styles.rows}>{informational.map((item, index) => <View key={item.id}>
        {index > 0 ? <SettingsSeparator /> : null}
        <NotificationRow
          busy={busy}
          i18n={i18n}
          item={item}
          onDelete={() => onDelete(item.id)}
          onOpen={canOpen(item) ? () => onOpen(item) : undefined}
        />
      </View>)}</View>
      <Text style={styles.footer}>{i18n.t('notification.informationalFooter')}</Text>
    </View> : null}
    {history.nextCursor ? <ActionButton
      disabled={busy}
      label={i18n.t(busy ? 'notification.loadingMore' : 'notification.loadMore')}
      onPress={onLoadMore}
      variant="secondary"
    /> : null}
    {errorKey ? <View style={styles.state}>
      <StatusBanner text={i18n.t(errorKey)} tone="error" />
      <ActionButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : null}
  </View>;
}

const styles = StyleSheet.create({
  date: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
  },
  footer: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  loading: {
    alignItems: 'center',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'center',
    minHeight: 220,
  },
  message: {
    color: mobileTheme.colors.text,
    flex: 1,
    fontSize: 16,
    lineHeight: 21,
  },
  messageLine: {
    alignItems: 'flex-start',
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
  },
  row: {
    alignItems: 'flex-start',
    flexDirection: 'row',
    paddingLeft: mobileTheme.spacing.md,
    paddingRight: mobileTheme.spacing.xs,
  },
  rowMain: {
    flex: 1,
    gap: mobileTheme.spacing.xxs,
    paddingRight: mobileTheme.spacing.xs,
    paddingVertical: mobileTheme.spacing.sm,
  },
  rowPressed: {
    opacity: 0.68,
  },
  sections: {
    gap: mobileTheme.spacing.lg,
  },
  section: {
    gap: mobileTheme.spacing.xxs,
  },
  sectionTitle: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    fontWeight: '600',
    lineHeight: 18,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  rows: {
    borderBottomColor: mobileTheme.colors.border,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderTopColor: mobileTheme.colors.border,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  state: {
    gap: mobileTheme.spacing.sm,
  },
  unreadDot: {
    backgroundColor: mobileTheme.colors.accent,
    borderRadius: 5,
    height: 9,
    marginTop: 6,
    width: 9,
  },
  unreadMessage: {
    fontWeight: '600',
  },
});
