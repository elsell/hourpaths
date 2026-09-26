import type {
  ManagedPendingPathInvitation,
  ManagedPendingPathInvitationState,
} from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { getLocales } from 'expo-localization';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import { NativeContentUnavailable } from './native-content-unavailable';
import { presentNativeDestructiveConfirmation } from './native-confirmation';
import {
  ActionButton,
  StatusBanner,
  ThemedText as Text,
} from './primitives';
import { SettingsActionRow, SettingsSection, SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

function ManagedInvitationRow({
  busy,
  errorKey,
  invitation,
  onCancelManagedInvitation,
  translator,
}: {
  busy: boolean;
  errorKey?: MessageKey;
  invitation: ManagedPendingPathInvitation;
  onCancelManagedInvitation: (invitation: ManagedPendingPathInvitation) => void;
  translator: Translator;
}) {
  const role = translator.t(
    invitation.invitation.offeredRole === 'participant'
      ? 'pathInvitation.role.participant'
      : 'pathInvitation.role.supporter',
  );
  const sentAt = new Date(invitation.invitation.createdAt);
  const recipient = translator.t('pathInvitation.managed.recipient', {
    displayName: invitation.recipient.displayName,
    username: invitation.recipient.username,
  });
  const inviter = translator.t('pathInvitation.managed.inviter', {
    displayName: invitation.inviter.displayName,
    username: invitation.inviter.username,
  });

  function confirmCancellation() {
    if (busy) return;
    presentNativeDestructiveConfirmation({
      cancelLabel: translator.t('common.cancel'),
      confirmLabel: translator.t('pathInvitation.managed.cancel'),
      message: translator.t('pathInvitation.managed.cancelConfirmationBody', {
        displayName: invitation.recipient.displayName,
        role: translator.t(`pathInvitation.role.${invitation.invitation.offeredRole}`),
        username: invitation.recipient.username,
      }),
      onConfirm: () => onCancelManagedInvitation(invitation),
      title: translator.t('pathInvitation.managed.cancelConfirmationHeading'),
    });
  }

  return <View>
    <View
      accessible
      accessibilityLabel={`${recipient}. ${translator.t('pathInvitation.managed.role', { role })}. ${inviter}. ${translator.t('pathInvitation.managed.sentAt', {
        date: translator.date(sentAt, { dateStyle: 'medium' }),
        time: translator.time(sentAt, { timeStyle: 'short' }),
      })}`}
      style={styles.managedDetails}
    >
      <Text style={styles.managedRecipient}>{recipient}</Text>
      <Text style={styles.managedMetadata}>{translator.t('pathInvitation.managed.role', { role })}</Text>
      <Text style={styles.managedMetadata}>{inviter}</Text>
      <Text style={styles.managedMetadata}>{translator.t('pathInvitation.managed.sentAt', {
        date: translator.date(sentAt, { dateStyle: 'medium' }),
        time: translator.time(sentAt, { timeStyle: 'short' }),
      })}</Text>
    </View>
    {errorKey ? <View style={styles.managedError}>
      <StatusBanner text={translator.t(errorKey)} tone="error" />
    </View> : null}
    <SettingsSeparator />
    <SettingsActionRow
      accessibilityLabel={translator.t(
        busy ? 'pathInvitation.managed.cancelingFor' : 'pathInvitation.managed.cancelFor',
        {
          displayName: invitation.recipient.displayName,
          role,
          username: invitation.recipient.username,
        },
      )}
      disabled={busy}
      label={translator.t(
        busy ? 'pathInvitation.managed.canceling' : 'pathInvitation.managed.cancel',
      )}
      onPress={confirmCancellation}
      tone="destructive"
    />
  </View>;
}

export function ManagedInvitations({
  busy,
  errorKey,
  invitationBusy,
  invitationErrors,
  invitations,
  onCancelManagedInvitation,
  onLoadMoreManagedInvitations,
  onRetryManagedInvitations,
}: {
  busy: boolean;
  errorKey?: MessageKey;
  invitationBusy: Readonly<Record<string, boolean | undefined>>;
  invitationErrors: Readonly<Record<string, MessageKey | undefined>>;
  invitations: ManagedPendingPathInvitationState;
  onCancelManagedInvitation: (invitation: ManagedPendingPathInvitation) => void;
  onLoadMoreManagedInvitations: () => void;
  onRetryManagedInvitations: () => void;
}) {
  if (busy && invitations.items.length === 0) {
    return <View
      accessibilityLabel={i18n.t('pathInvitation.managed.loading')}
      accessibilityRole="progressbar"
      style={styles.managedState}
    >
      <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      <Text style={styles.managedMetadata}>{i18n.t('pathInvitation.managed.loading')}</Text>
    </View>;
  }

  if (errorKey && invitations.items.length === 0) {
    return <View style={styles.managedState}>
      <NativeContentUnavailable
        description={i18n.t(errorKey)}
        systemImage="person.badge.clock"
        title={i18n.t('pathInvitation.managed.unavailableHeading')}
      />
      <ActionButton
        label={i18n.t('common.retry')}
        onPress={onRetryManagedInvitations}
        variant="secondary"
      />
    </View>;
  }

  if (invitations.items.length === 0) {
    return <NativeContentUnavailable
      description={i18n.t('pathInvitation.managed.emptyDescription')}
      systemImage="person.badge.plus"
      title={i18n.t('pathInvitation.managed.emptyHeading')}
    />;
  }

  return <View style={styles.managedSections}>
    <SettingsSection
      title={i18n.t('pathInvitation.managed.count', { count: invitations.items.length })}
    >
      {invitations.items.map((invitation, index) => <View key={invitation.invitation.id}>
        {index > 0 ? <SettingsSeparator /> : null}
        <ManagedInvitationRow
          busy={Boolean(invitationBusy[invitation.invitation.id])}
          errorKey={invitationErrors[invitation.invitation.id]}
          invitation={invitation}
          onCancelManagedInvitation={onCancelManagedInvitation}
          translator={i18n}
        />
      </View>)}
    </SettingsSection>
    {invitations.nextCursor ? <ActionButton
      busy={busy}
      disabled={busy}
      label={i18n.t(busy ? 'pathInvitation.managed.loadingMore' : 'common.loadMore')}
      onPress={onLoadMoreManagedInvitations}
      variant="secondary"
    /> : null}
    {errorKey ? <StatusBanner
      actionLabel={i18n.t('common.retry')}
      onAction={onRetryManagedInvitations}
      text={i18n.t(errorKey)}
      tone="error"
    /> : null}
  </View>;
}

const styles = StyleSheet.create({
  managedDetails: {
    gap: mobileTheme.spacing.xxs,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  managedError: {
    paddingHorizontal: mobileTheme.spacing.md,
    paddingBottom: mobileTheme.spacing.sm,
  },
  managedMetadata: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
  },
  managedRecipient: {
    color: mobileTheme.colors.text,
    fontSize: 17,
    fontWeight: '600',
    lineHeight: 22,
  },
  managedSections: {
    gap: mobileTheme.spacing.md,
  },
  managedState: {
    alignItems: 'center',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'center',
    minHeight: 180,
  },
});
