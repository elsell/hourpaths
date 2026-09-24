import { useEffect, useState } from 'react';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { ActivityIndicator, Pressable, StyleSheet, View } from 'react-native';
import {
  ownershipTransferActionState,
  reconcileOwnershipTransferScreen,
  type OwnershipTransferScreen,
} from '../ownership-transfer-presentation';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeSheet, ThemedText as Text } from './primitives';
import {
  SettingsActionRow,
  SettingsNavigationRow,
  SettingsSection,
  SettingsSeparator,
  SettingsValueRow,
} from './settings-list';
import { mobileTheme } from './tokens';

export type OwnershipTransferCandidate = {
  displayName: string;
  isAdministrator: boolean;
  userID: string;
  username?: string;
};

export type OwnershipTransferCandidatePage = {
  errorKey?: MessageKey;
  items: readonly OwnershipTransferCandidate[];
  loading: boolean;
  nextCursor?: string;
};

export type PendingOwnershipTransfer = {
  counterpart: OwnershipTransferCandidate;
  expirationSummary: string;
  id: string;
  viewerRole: 'creator' | 'recipient';
};

export type OwnershipTransferBusyAction = 'accept' | 'cancel' | 'confirm' | 'decline';

export type OwnershipTransferSheetProps = {
  actionsDisabled?: boolean;
  busy: boolean;
  busyAction?: OwnershipTransferBusyAction;
  candidates: OwnershipTransferCandidatePage;
  errorText?: string;
  expirationSummary?: string;
  i18n: Translator;
  onAcceptPending: (transfer: PendingOwnershipTransfer) => void;
  onCancelPending: (transfer: PendingOwnershipTransfer) => void;
  onClose: () => void;
  onConfirm: (recipient: OwnershipTransferCandidate) => void;
  onDeclinePending: (transfer: PendingOwnershipTransfer) => void;
  onLoadCandidates: () => void;
  onLoadMoreCandidates: () => void;
  onSelectRecipient: (recipient: OwnershipTransferCandidate) => void;
  pending?: PendingOwnershipTransfer;
  selectedRecipientID?: string;
  visible: boolean;
};

export function OwnershipTransferSheet({
  actionsDisabled = false,
  busy,
  busyAction,
  candidates,
  errorText,
  expirationSummary,
  i18n,
  onAcceptPending,
  onCancelPending,
  onClose,
  onConfirm,
  onDeclinePending,
  onLoadCandidates,
  onLoadMoreCandidates,
  onSelectRecipient,
  pending,
  selectedRecipientID,
  visible,
}: OwnershipTransferSheetProps) {
  const [screen, setScreen] = useState<OwnershipTransferScreen>(pending ? 'pending' : 'overview');
  const selected = candidates.items.find(({ userID }) => userID === selectedRecipientID);
  const actionState = ownershipTransferActionState({ busy, selectedRecipientID });

  useEffect(() => {
    setScreen((current) => reconcileOwnershipTransferScreen(current, {
      hasPendingTransfer: Boolean(pending),
    }));
  }, [pending]);

  function back() {
    if (busy) return;
    setScreen(screen === 'review' ? 'candidates' : 'overview');
  }

  const title = screen === 'candidates'
    ? i18n.t('pathOwnership.selectRecipient')
    : screen === 'review'
      ? i18n.t('pathOwnership.reviewHeading')
      : screen === 'pending'
        ? i18n.t('pathOwnership.pendingHeading')
        : i18n.t('pathOwnership.heading');

  return <NativeSheet
    compact
    dismissible={!busy}
    leadingAction={screen === 'overview' || screen === 'pending' ? undefined : {
      disabled: busy,
      label: i18n.t('common.back'),
      onPress: back,
    }}
    onRequestClose={onClose}
    title={title}
    trailingAction={screen === 'overview' || screen === 'pending' ? {
      disabled: busy,
      label: i18n.t('common.done'),
      onPress: onClose,
    } : screen === 'review' ? {
      disabled: !actionState.canConfirm,
      label: i18n.t(busyAction === 'confirm' ? 'pathOwnership.sending' : 'pathOwnership.confirm'),
      onPress: () => {
        if (selected && actionState.canConfirm) onConfirm(selected);
      },
    } : undefined}
    visible={visible}
  >
    {screen === 'overview' ? <SettingsSection footer={i18n.t('pathOwnership.explanation')}>
      <SettingsNavigationRow
        accessibilityLabel={i18n.t('pathOwnership.selectRecipient')}
        context={i18n.t('pathOwnership.noChangeUntilAccepted')}
        label={i18n.t('pathOwnership.selectRecipient')}
        onPress={() => {
          setScreen('candidates');
          if (candidates.items.length === 0 && !candidates.loading) onLoadCandidates();
        }}
        value={selected?.displayName}
      />
    </SettingsSection> : null}

    {screen === 'candidates' ? <CandidateList
      busy={busy}
      candidates={candidates}
      i18n={i18n}
      onLoadMoreCandidates={onLoadMoreCandidates}
      onRetry={onLoadCandidates}
      onSelect={(candidate) => {
        onSelectRecipient(candidate);
        setScreen('review');
      }}
      selectedRecipientID={selectedRecipientID}
    /> : null}

    {screen === 'review' && selected ? <>
      <SettingsSection title={i18n.t('pathOwnership.reviewRecipient')}>
        <CandidateDetails candidate={selected} i18n={i18n} />
      </SettingsSection>
      <SettingsSection footer={i18n.t('pathOwnership.noChangeUntilAccepted')}>
        <SettingsValueRow
          label={i18n.t('pathOwnership.recipientRole')}
          value={i18n.t('pathOwnership.creator')}
        />
        <SettingsSeparator />
        <SettingsValueRow
          label={i18n.t('pathOwnership.yourRole')}
          value={i18n.t('pathOwnership.administrator')}
        />
      </SettingsSection>
      <View style={styles.reviewNotice}>
        <Text style={styles.noticeTitle}>{i18n.t('pathOwnership.recipientBecomesCreator')}</Text>
        <Text style={styles.noticeText}>{i18n.t('pathOwnership.creatorBecomesAdministrator')}</Text>
      </View>
      {expirationSummary ? <Text style={styles.expiration}>{expirationSummary}</Text> : null}
    </> : null}

    {screen === 'pending' && pending ? <PendingTransfer
      actionsDisabled={actionsDisabled}
      busy={busy}
      busyAction={busyAction}
      i18n={i18n}
      onAccept={() => onAcceptPending(pending)}
      onCancel={() => onCancelPending(pending)}
      onDecline={() => onDeclinePending(pending)}
      pending={pending}
    /> : null}
    {errorText ? <View accessibilityRole="alert" style={styles.inlineError}>
      <Text style={styles.error}>{errorText}</Text>
    </View> : null}
  </NativeSheet>;
}

function CandidateList({
  busy,
  candidates,
  i18n,
  onLoadMoreCandidates,
  onRetry,
  onSelect,
  selectedRecipientID,
}: {
  busy: boolean;
  candidates: OwnershipTransferCandidatePage;
  i18n: Translator;
  onLoadMoreCandidates: () => void;
  onRetry: () => void;
  onSelect: (candidate: OwnershipTransferCandidate) => void;
  selectedRecipientID?: string;
}) {
  if (candidates.loading && candidates.items.length === 0) return <View
    accessibilityLabel={i18n.t('pathOwnership.loadingCandidates')}
    accessibilityRole="progressbar"
    style={styles.state}
  >
    <ActivityIndicator color={mobileTheme.colors.accent} size="large" />
    <Text style={styles.secondary}>{i18n.t('pathOwnership.loadingCandidates')}</Text>
  </View>;

  if (candidates.errorKey && candidates.items.length === 0) return <View style={styles.state}>
    <NativeContentUnavailable
      description={i18n.t(candidates.errorKey)}
      systemImage="person.2.slash"
      title={i18n.t('pathOwnership.candidatesUnavailable')}
    />
    <SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t('common.retry')}
        disabled={busy}
        label={i18n.t('common.retry')}
        onPress={onRetry}
        tone="default"
      />
    </SettingsSection>
  </View>;

  if (candidates.items.length === 0) return <NativeContentUnavailable
    description={i18n.t('pathOwnership.candidatesEmptyDescription')}
    systemImage="person.2"
    title={i18n.t('pathOwnership.candidatesEmpty')}
  />;

  return <View style={styles.sections}>
    <SettingsSection footer={i18n.t('pathOwnership.candidateFooter')}>
      {candidates.items.map((candidate, index) => <View key={candidate.userID}>
        {index > 0 ? <SettingsSeparator /> : null}
        <CandidateRow
          busy={busy}
          candidate={candidate}
          i18n={i18n}
          onPress={() => onSelect(candidate)}
          selected={candidate.userID === selectedRecipientID}
        />
      </View>)}
    </SettingsSection>
    {candidates.nextCursor ? <SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t('common.loadMore')}
        disabled={busy}
        label={i18n.t(candidates.loading ? 'pathOwnership.loadingMore' : 'common.loadMore')}
        onPress={onLoadMoreCandidates}
        tone="default"
      />
    </SettingsSection> : null}
    {candidates.errorKey ? <View accessibilityRole="alert" style={styles.inlineError}>
      <Text style={styles.error}>{i18n.t(candidates.errorKey)}</Text>
    </View> : null}
  </View>;
}

function CandidateRow({ busy, candidate, i18n, onPress, selected }: {
  busy: boolean;
  candidate: OwnershipTransferCandidate;
  i18n: Translator;
  onPress: () => void;
  selected: boolean;
}) {
  return <Pressable
    accessibilityLabel={i18n.t('pathOwnership.candidateAccessibility', { name: candidate.displayName })}
    accessibilityRole="radio"
    accessibilityState={{ disabled: busy, selected }}
    disabled={busy}
    onPress={onPress}
    style={({ pressed }) => [styles.candidateRow, pressed ? styles.pressed : null]}
  >
    <CandidateDetails candidate={candidate} i18n={i18n} compact />
    <Text style={selected ? styles.selected : styles.unselected}>{selected ? '✓' : '›'}</Text>
  </Pressable>;
}

function CandidateDetails({ candidate, compact = false, i18n }: {
  candidate: OwnershipTransferCandidate;
  compact?: boolean;
  i18n: Translator;
}) {
  return <View style={[styles.identity, compact ? styles.identityCompact : null]}>
    <Text numberOfLines={1} style={styles.name}>{candidate.displayName}</Text>
    {candidate.username ? <Text numberOfLines={1} style={styles.secondary}>@{candidate.username}</Text> : null}
    {candidate.isAdministrator ? <Text style={styles.role}>{i18n.t('pathOwnership.administrator')}</Text> : null}
  </View>;
}

function PendingTransfer({ actionsDisabled, busy, busyAction, i18n, onAccept, onCancel, onDecline, pending }: {
  actionsDisabled: boolean;
  busy: boolean;
  busyAction?: OwnershipTransferBusyAction;
  i18n: Translator;
  onAccept: () => void;
  onCancel: () => void;
  onDecline: () => void;
  pending: PendingOwnershipTransfer;
}) {
  return <>
    <SettingsSection
      footer={i18n.t(pending.viewerRole === 'creator'
        ? 'pathOwnership.creatorPendingExplanation'
        : 'pathOwnership.recipientPendingExplanation')}
      title={i18n.t('pathOwnership.pendingWith')}
    >
      <CandidateDetails candidate={pending.counterpart} i18n={i18n} />
      <SettingsSeparator />
      <SettingsValueRow label={i18n.t('pathOwnership.expires')} value={pending.expirationSummary} />
    </SettingsSection>
    {pending.viewerRole === 'creator' ? <SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t(busyAction === 'cancel' ? 'pathOwnership.canceling' : 'pathOwnership.cancel')}
        disabled={busy || actionsDisabled}
        label={i18n.t(busyAction === 'cancel' ? 'pathOwnership.canceling' : 'pathOwnership.cancel')}
        onPress={onCancel}
        tone="destructive"
      />
    </SettingsSection> : null}
    {pending.viewerRole === 'recipient' ? <SettingsSection>
      <SettingsActionRow
        accessibilityLabel={i18n.t(busyAction === 'accept' ? 'pathOwnership.accepting' : 'pathOwnership.accept')}
        disabled={busy || actionsDisabled}
        label={i18n.t(busyAction === 'accept' ? 'pathOwnership.accepting' : 'pathOwnership.accept')}
        onPress={onAccept}
        tone="default"
      />
      <SettingsSeparator />
      <SettingsActionRow
        accessibilityLabel={i18n.t(busyAction === 'decline' ? 'pathOwnership.declining' : 'pathOwnership.decline')}
        disabled={busy || actionsDisabled}
        label={i18n.t(busyAction === 'decline' ? 'pathOwnership.declining' : 'pathOwnership.decline')}
        onPress={onDecline}
        tone="destructive"
      />
    </SettingsSection> : null}
  </>;
}

const styles = StyleSheet.create({
  candidateRow: {
    alignItems: 'center',
    flexDirection: 'row',
    minHeight: 58,
    paddingHorizontal: mobileTheme.spacing.md,
    paddingVertical: mobileTheme.spacing.sm,
  },
  error: { color: mobileTheme.colors.error, fontSize: 14, lineHeight: 20 },
  expiration: {
    color: mobileTheme.colors.textMuted,
    fontSize: 13,
    lineHeight: 18,
    paddingHorizontal: mobileTheme.spacing.md,
  },
  identity: { flex: 1, gap: mobileTheme.spacing.xxs, minWidth: 0, padding: mobileTheme.spacing.md },
  identityCompact: { padding: 0 },
  inlineError: { paddingHorizontal: mobileTheme.spacing.md },
  name: { color: mobileTheme.colors.text, fontSize: 17, fontWeight: '500', lineHeight: 22 },
  noticeText: { color: mobileTheme.colors.textMuted, fontSize: 14, lineHeight: 20 },
  noticeTitle: { color: mobileTheme.colors.text, fontSize: 16, fontWeight: '600', lineHeight: 21 },
  pressed: { backgroundColor: mobileTheme.colors.surfacePressed },
  reviewNotice: {
    backgroundColor: mobileTheme.colors.surface,
    borderRadius: mobileTheme.radii.md,
    gap: mobileTheme.spacing.xs,
    padding: mobileTheme.spacing.md,
  },
  role: { color: mobileTheme.colors.accent, fontSize: 13, fontWeight: '600', lineHeight: 18 },
  secondary: { color: mobileTheme.colors.textMuted, fontSize: 14, lineHeight: 20 },
  sections: { gap: mobileTheme.spacing.lg },
  selected: { color: mobileTheme.colors.accent, fontSize: 22, fontWeight: '700' },
  state: { alignItems: 'stretch', gap: mobileTheme.spacing.md, justifyContent: 'center', minHeight: 220 },
  unselected: { color: mobileTheme.colors.textMuted, fontSize: 28, fontWeight: '300' },
});
