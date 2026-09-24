import { useEffect, useRef, useState } from 'react';
import type { ManagedPendingPathInvitation, ManagedPendingPathInvitationState, PathVisibility } from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { AccessibilityInfo, StyleSheet, View } from 'react-native';
import { NativeChoicePicker } from './native-choice-picker';
import { manualShareDraftChanged, type PathShareDraft } from './path-share-draft';
import { ManagedInvitations } from './path-invitation-sheet';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeSheet, StatusBanner, ThemedText as Text, ThemedTextInput } from './primitives';
import { SettingsActionRow, SettingsNavigationRow, SettingsSection, SettingsSeparator, SettingsValueRow } from './settings-list';
import { mobileTheme } from './tokens';

export type PathSharePerson = Readonly<{ canManage: boolean; displayName: string; role: 'creator' | 'administrator' | 'participant' | 'supporter'; userID: string; username: string }>;
type Role = 'participant' | 'supporter';

export type PathShareSheetProps = {
  busy: boolean; canInvite: boolean; effectiveVisibility: PathVisibility; errorText?: string; i18n: Translator;
  managedInvitationBusy: Readonly<Record<string, boolean | undefined>>;
  managedInvitationErrors: Readonly<Record<string, MessageKey | undefined>>;
  managedInvitationsBusy: boolean; managedInvitationsErrorKey?: MessageKey;
  onCancel: (dirty: boolean) => void; onChangeRole: (role: Role) => void;
  onCancelManagedInvitation: (invitation: ManagedPendingPathInvitation) => void;
  onChangeUsername: (value: string) => void; onInvite: () => void; onReview: () => void;
  onLoadMoreManagedInvitations: () => void; onRetryManagedInvitations: () => void;
  onLoadMorePeople: () => void; onOpenPerson: (userID: string) => void; onRetryPeople: () => void;
  onVisibilityChange?: (visibility: PathVisibility) => void; people: readonly PathSharePerson[];
  peopleError: boolean; peopleLoading: boolean; peopleLoadingMore: boolean; peopleNextCursor?: string;
  pendingInvitations: ManagedPendingPathInvitationState;
  review?: Readonly<{ recipient: Readonly<{ displayName: string; username: string }> }> | null;
  reviewed: boolean; role: Role; sent: boolean;
  username: string; visibilityOptions?: readonly PathVisibility[];
};

export function PathShareSheet(props: PathShareSheetProps) {
  const [screen, setScreen] = useState<'overview' | 'invite'>('overview');
  const [reduceMotion, setReduceMotion] = useState<boolean | null>(null);
  const baseline = useRef<PathShareDraft>({ role: props.role, username: props.username });
  const dirty = screen === 'invite' && manualShareDraftChanged(baseline.current, { role: props.role, username: props.username });
  useEffect(() => {
    void AccessibilityInfo.isReduceMotionEnabled().then(setReduceMotion);
    const listener = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    return () => listener.remove();
  }, []);
  useEffect(() => {
    if (props.sent) baseline.current = { role: props.role, username: props.username };
  }, [props.role, props.sent, props.username]);
  const close = () => props.onCancel(dirty);
  const action = screen === 'overview'
    ? props.canInvite
      ? { disabled: props.busy, label: props.i18n.t('pathInvitation.inviteAction'), onPress: () => { baseline.current = { role: props.role, username: props.username }; setScreen('invite'); } }
      : undefined
    : { disabled: props.busy || !props.username.trim(), label: props.i18n.t(props.reviewed ? 'pathInvitation.send' : 'pathInvitation.review'), onPress: props.reviewed ? props.onInvite : props.onReview };
  return <NativeSheet animationType={reduceMotion === false ? 'slide' : 'none'} compact
    dismissible={!props.busy && !dirty} leadingAction={{ disabled: props.busy, label: props.i18n.t('common.cancel'), onPress: close }}
    onRequestClose={close} title={props.i18n.t('pathInvitation.heading')} trailingAction={action} visible>
    {screen === 'overview' ? <>
      <SettingsSection title={props.i18n.t('pathVisibility.heading')}>
        {props.onVisibilityChange && props.visibilityOptions ? <NativeChoicePicker accessibilityLabel={props.i18n.t('pathVisibility.choiceLabel')}
          choices={props.visibilityOptions.map((value) => ({ label: props.i18n.t(`pathVisibility.option.${value}`), value }))}
          disabled={props.busy} label={props.i18n.t('pathVisibility.choiceLabel')} onChange={props.onVisibilityChange} value={props.effectiveVisibility} />
          : <SettingsValueRow label={props.i18n.t('pathVisibility.heading')} value={props.i18n.t(`pathVisibility.option.${props.effectiveVisibility}`)} />}
      </SettingsSection>
      {props.peopleLoading && props.people.length === 0
        ? <StatusBanner text={props.i18n.t('pathMembers.loading')} tone="loading" />
        : props.peopleError && props.people.length === 0
          ? <View style={styles.form}><NativeContentUnavailable description={props.i18n.t('pathMembers.unavailableDescription')} systemImage="person.2.slash" title={props.i18n.t('pathMembers.unavailableHeading')} />
            <SettingsSection><SettingsActionRow accessibilityLabel={props.i18n.t('common.retry')} label={props.i18n.t('common.retry')} onPress={props.onRetryPeople} /></SettingsSection></View>
          : <SettingsSection title={props.i18n.t('pathMembers.heading')}>{props.people.map((person, index) => {
            const roleLabel = props.i18n.t(person.role === 'administrator'
              ? 'pathOwnership.administrator'
              : `pathMembers.${person.role}`);
            const identityLabel = props.i18n.t('pathMembers.identityWithRole', {
              displayName: person.displayName,
              role: roleLabel,
              username: person.username,
            });
            return <View key={person.userID}>
              {index ? <SettingsSeparator /> : null}
              {person.canManage ? <SettingsNavigationRow accessibilityLabel={identityLabel}
                label={person.displayName} onPress={() => props.onOpenPerson(person.userID)} value={props.i18n.t('pathMembers.usernameWithRole', {
                  role: roleLabel,
                  username: person.username,
                })} />
                : <View accessible accessibilityLabel={identityLabel} style={styles.identity}>
                  <Text style={styles.name}>{person.displayName}</Text><Text style={styles.secondary}>@{person.username}</Text><Text style={styles.secondary}>{roleLabel}</Text>
                </View>}
            </View>;
          })}</SettingsSection>}
      {props.peopleNextCursor ? <SettingsSection><SettingsActionRow accessibilityLabel={props.i18n.t(props.peopleLoadingMore ? 'common.loading' : 'common.loadMore')} disabled={props.peopleLoadingMore}
        label={props.i18n.t(props.peopleLoadingMore ? 'common.loading' : 'common.loadMore')} onPress={props.onLoadMorePeople} /></SettingsSection> : null}
      {props.peopleError && props.people.length > 0 ? <StatusBanner actionLabel={props.i18n.t('common.retry')} onAction={props.onRetryPeople}
        text={props.i18n.t('pathMembers.unavailableDescription')} tone="error" /> : null}
      {props.canInvite ? <ManagedInvitations
        busy={props.managedInvitationsBusy}
        errorKey={props.managedInvitationsErrorKey}
        invitationBusy={props.managedInvitationBusy}
        invitationErrors={props.managedInvitationErrors}
        invitations={props.pendingInvitations}
        onCancelManagedInvitation={props.onCancelManagedInvitation}
        onLoadMoreManagedInvitations={props.onLoadMoreManagedInvitations}
        onRetryManagedInvitations={props.onRetryManagedInvitations}
      /> : null}
    </> : <View style={styles.form}>
      <ThemedTextInput accessibilityLabel={props.i18n.t('pathInvitation.usernameLabel')} autoCapitalize="none" autoCorrect={false}
        editable={!props.busy} enterKeyHint="done" onChangeText={props.onChangeUsername} onSubmitEditing={props.reviewed ? props.onInvite : props.onReview}
        returnKeyType="done" value={props.username} />
      <Text style={styles.secondary}>{props.i18n.t('pathInvitation.usernameHint')}</Text>
      <NativeChoicePicker accessibilityLabel={props.i18n.t('pathInvitation.roleLabel')} choices={(['participant', 'supporter'] as const).map((value) => ({ label: props.i18n.t(`pathInvitation.role.${value}`), value }))}
        disabled={props.busy} label={props.i18n.t('pathInvitation.roleLabel')} onChange={props.onChangeRole} value={props.role} />
      <Text style={styles.secondary}>{props.i18n.t(props.role === 'participant'
        ? 'pathInvitation.role.participantEffect'
        : 'pathInvitation.role.supporterEffect')}</Text>
      {props.review ? <SettingsSection title={props.i18n.t('pathInvitation.confirmHeading')}>
        <SettingsValueRow label={props.i18n.t('pathInvitation.usernameLabel')} value={props.i18n.t('pathInvitation.reviewedIdentity', {
          displayName: props.review.recipient.displayName,
          username: props.review.recipient.username,
        })} />
        <Text style={styles.empty}>{props.i18n.t('pathInvitation.confirmSend', {
          displayName: props.review.recipient.displayName,
          role: props.i18n.t(`pathInvitation.role.${props.role}`),
          username: props.review.recipient.username,
        })}</Text>
      </SettingsSection> : null}
      {props.sent && props.review ? <Text accessibilityLiveRegion="polite" style={styles.secondary}>{props.i18n.t('pathInvitation.sent', {
        displayName: props.review.recipient.displayName,
        username: props.review.recipient.username,
      })}</Text> : null}
      {props.errorText ? <StatusBanner text={props.errorText} tone="error" /> : null}
      {props.busy ? <StatusBanner text={props.i18n.t('common.loading')} tone="loading" /> : null}
      <SettingsSection><SettingsActionRow accessibilityLabel={props.i18n.t('common.back')} label={props.i18n.t('common.back')}
        onPress={() => { if (dirty) close(); else setScreen('overview'); }} /></SettingsSection>
    </View>}
  </NativeSheet>;
}
const styles = StyleSheet.create({ empty: { color: mobileTheme.colors.textMuted, padding: mobileTheme.spacing.md }, form: { gap: mobileTheme.spacing.md }, identity: { gap: mobileTheme.spacing.xxs, padding: mobileTheme.spacing.md }, name: { fontSize: 17, fontWeight: '600' }, secondary: { color: mobileTheme.colors.textMuted } });
