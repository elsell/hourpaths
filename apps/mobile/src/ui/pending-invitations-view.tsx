import type {
  PathInvitationRole,
  PendingPathInvitation,
  PendingPathInvitationState,
} from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { ActionButton, StatusBanner, ThemedText as Text } from './primitives';
import { SettingsActionRow, SettingsSeparator } from './settings-list';
import { mobileTheme } from './tokens';

function PendingInvitationRow({
  busy,
  errorKey,
  focused,
  i18n,
  invitation,
  onAccept,
  onReject,
  rejecting,
}: {
  busy: boolean;
  errorKey?: MessageKey;
  focused: boolean;
  i18n: Translator;
  invitation: PendingPathInvitation;
  onAccept: () => void;
  onReject: () => void;
  rejecting: boolean;
}) {
  const roleKey = invitation.invitation.offeredRole === 'participant'
    ? 'pathInvitation.role.participant'
    : 'pathInvitation.role.supporter';
  const effectKey = invitation.invitation.offeredRole === 'participant'
    ? 'pathInvitation.role.participantEffect'
    : 'pathInvitation.role.supporterEffect';

  return <View style={focused ? styles.focused : null}>
    <View
      accessibilityLabel={i18n.t('pathInvitation.pendingContext', {
        displayName: invitation.inviter.displayName,
        pathName: invitation.pathName,
        username: invitation.inviter.username,
      })}
      style={styles.details}
    >
      <Text accessibilityRole="header" style={styles.pathName}>{invitation.pathName}</Text>
      <Text style={styles.inviter}>{i18n.t('pathInvitation.reviewedIdentity', {
        displayName: invitation.inviter.displayName,
        username: invitation.inviter.username,
      })}</Text>
      <Text style={styles.role}>{i18n.t(roleKey)}</Text>
      <Text style={styles.effect}>{i18n.t(effectKey)}</Text>
      {errorKey ? <StatusBanner text={i18n.t(errorKey)} tone="error" /> : null}
    </View>
    <SettingsSeparator />
    <SettingsActionRow
      accessibilityLabel={i18n.t(busy && !rejecting ? 'pathInvitation.accepting' : 'pathInvitation.accept')}
      disabled={busy}
      label={i18n.t(busy && !rejecting ? 'pathInvitation.accepting' : 'pathInvitation.accept')}
      onPress={onAccept}
      tone="default"
    />
    <SettingsSeparator />
    <SettingsActionRow
      accessibilityLabel={i18n.t(rejecting ? 'pathInvitation.rejecting' : 'pathInvitation.reject')}
      disabled={busy}
      label={i18n.t(rejecting ? 'pathInvitation.rejecting' : 'pathInvitation.reject')}
      onPress={onReject}
      tone="destructive"
    />
  </View>;
}

export function PendingInvitationsView({
  acceptedRole,
  busy,
  errorKey,
  focusedInvitationID,
  invitationBusy,
  invitationErrors,
  invitationRejecting,
  invitations,
  i18n,
  onAccept,
  onReject,
  onLoadMore,
  onRetry,
  rejected,
}: {
  acceptedRole?: PathInvitationRole;
  busy: boolean;
  errorKey?: MessageKey;
  focusedInvitationID?: string;
  invitationBusy: Readonly<Record<string, boolean | undefined>>;
  invitationErrors: Readonly<Record<string, MessageKey | undefined>>;
  invitationRejecting: Readonly<Record<string, boolean | undefined>>;
  invitations: PendingPathInvitationState;
  i18n: Translator;
  onAccept: (invitation: PendingPathInvitation) => void;
  onReject: (invitation: PendingPathInvitation) => void;
  onLoadMore: () => void;
  onRetry: () => void;
  rejected: boolean;
}) {
  const orderedInvitations = focusedInvitationID
    ? [
        ...invitations.items.filter((item) => item.invitation.id === focusedInvitationID),
        ...invitations.items.filter((item) => item.invitation.id !== focusedInvitationID),
      ]
    : invitations.items;

  if (busy && invitations.items.length === 0) {
    return <View
      accessibilityLabel={i18n.t('pathInvitation.loading')}
      accessibilityRole="progressbar"
      style={styles.loading}
    >
      <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
      <Text style={styles.inviter}>{i18n.t('pathInvitation.loading')}</Text>
    </View>;
  }

  if (errorKey && invitations.items.length === 0) {
    return <View style={styles.state}>
      <NativeContentUnavailable
        description={i18n.t(errorKey)}
        systemImage="person.2.slash"
        title={i18n.t('pathInvitation.unavailableHeading')}
      />
      <ActionButton label={i18n.t('common.retry')} onPress={onRetry} variant="secondary" />
    </View>;
  }

  if (invitations.items.length === 0) {
    return <View style={styles.state}>
      <NativeContentUnavailable
        description={i18n.t('pathInvitation.pendingEmptyDescription')}
        systemImage="person.2"
        title={i18n.t('pathInvitation.pendingEmpty')}
      />
      {acceptedRole ? <Text accessibilityLiveRegion="polite" style={styles.success}>
        {i18n.t('pathInvitation.accepted', { role: i18n.t(
          acceptedRole === 'participant'
            ? 'pathInvitation.role.participant'
            : 'pathInvitation.role.supporter',
        ) })}
      </Text> : null}
      {rejected ? <Text accessibilityLiveRegion="polite" style={styles.success}>
        {i18n.t('pathInvitation.rejected')}
      </Text> : null}
    </View>;
  }

  return <View style={styles.sections}>
    <View style={styles.section}>
      <Text accessibilityRole="header" style={styles.sectionTitle}>
        {i18n.t('pathInvitation.pendingCount', { count: invitations.items.length })}
      </Text>
      <View style={styles.rows}>{orderedInvitations.map((invitation, index) => <View key={invitation.invitation.id}>
        {index > 0 ? <SettingsSeparator /> : null}
        <PendingInvitationRow
          busy={Boolean(invitationBusy[invitation.invitation.id])}
          errorKey={invitationErrors[invitation.invitation.id]}
          focused={focusedInvitationID === invitation.invitation.id}
          i18n={i18n}
          invitation={invitation}
          onAccept={() => onAccept(invitation)}
          onReject={() => onReject(invitation)}
          rejecting={Boolean(invitationRejecting[invitation.invitation.id])}
        />
      </View>)}</View>
      <Text style={styles.footer}>{i18n.t('pathInvitation.pendingFooter')}</Text>
    </View>
    {invitations.nextCursor ? <ActionButton
      disabled={busy}
      label={i18n.t('common.loadMore')}
      onPress={onLoadMore}
      variant="secondary"
    /> : null}
    {errorKey ? <View style={styles.state}>
      <StatusBanner text={i18n.t(errorKey)} tone="error" />
      <ActionButton label={i18n.t('common.retry')} onPress={onRetry} variant="quiet" />
    </View> : null}
    {acceptedRole ? <Text accessibilityLiveRegion="polite" style={styles.success}>
      {i18n.t(acceptedRole === 'participant'
        ? 'pathInvitation.participantTracking'
        : 'pathInvitation.supporterReadOnly')}
    </Text> : null}
    {rejected ? <Text accessibilityLiveRegion="polite" style={styles.success}>
      {i18n.t('pathInvitation.rejected')}
    </Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  effect: {
    color: mobileTheme.colors.textMuted,
    fontSize: 14,
    lineHeight: 20,
  },
  focused: {
    backgroundColor: mobileTheme.colors.surfacePressed,
  },
  footer: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  inviter: {
    color: mobileTheme.colors.textMuted,
    fontSize: 14,
    lineHeight: 20,
  },
  loading: {
    alignItems: 'center',
    gap: mobileTheme.spacing.sm,
    justifyContent: 'center',
    minHeight: 220,
  },
  pathName: {
    color: mobileTheme.colors.text,
    fontSize: 17,
    fontWeight: '600',
    lineHeight: 22,
  },
  role: {
    color: mobileTheme.colors.accent,
    fontSize: 14,
    fontWeight: '600',
    lineHeight: 20,
  },
  details: {
    gap: mobileTheme.spacing.xs,
    padding: mobileTheme.spacing.md,
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
  success: {
    color: mobileTheme.colors.text,
    fontSize: 15,
    lineHeight: 21,
    paddingHorizontal: mobileTheme.spacing.md,
  },
});
