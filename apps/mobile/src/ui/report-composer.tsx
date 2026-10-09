import { createReportSubmissionOwner, reportReasons, ReportFailure, type ReportReason, type ReportReceipt, type ReportTarget, type BlockReview, type UserBlockingPort } from '@hourpaths/client-core';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import * as Crypto from 'expo-crypto';
import { StyleSheet, View } from 'react-native';
import { NativeSheet, ThemedText as Text, ThemedTextInput } from './primitives';
import { NativeButton } from './native-button';
import { NativeActionMenu } from './native-action-menu';
import { SettingsChoiceRow } from './settings-choice-row';
import { SettingsSection, SettingsSeparator } from './settings-list';
import { useSettingsPresentation, type SettingsPresentation } from './settings-presentation';
import { useUserBlockingRoutePresentation } from './user-blocking-route-presentation';
import { mobileTheme } from './tokens';

export function useReportAction(target: ReportTarget, i18n: Translator) {
  const presentation = useSettingsPresentation();
  const [opened, setOpened] = useState<{ target: ReportTarget; sessionKey: string } | null>(null);
  const admitted = !!presentation?.isCurrent();
  const action = { label: i18n.t('reporting.action'), systemImage: 'flag' as const, disabled: !admitted || !target.id, onPress: () => { if (target.id && presentation?.isCurrent()) setOpened({ target: { ...target }, sessionKey: presentation.sessionKey }); } };
  return { action, sheet: opened && presentation && opened.sessionKey === presentation.sessionKey && admitted
    ? <ReportComposer key={JSON.stringify([opened.sessionKey, opened.target.kind, opened.target.id])} target={opened.target} i18n={i18n} presentation={presentation} close={() => setOpened(null)} /> : null };
}
export function ReportMenu({ target, i18n }: { target: ReportTarget; i18n: Translator }) {
  const report = useReportAction(target, i18n);
  return <><NativeActionMenu accessibilityLabel={i18n.t('reporting.action')} actions={[report.action]} />{report.sheet}</>;
}

function ReportComposer({ target, i18n, presentation, close }: { target: ReportTarget; i18n: Translator; presentation: SettingsPresentation; close(): void }) {
  const [owner] = useState(() => createReportSubmissionOwner(Crypto.randomUUID));
  const blocking = useUserBlockingRoutePresentation();
  const [reason, setReason] = useState<ReportReason | null>(null), [explanation, setExplanation] = useState('');
  const [receipt, setReceipt] = useState<ReportReceipt | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState<MessageKey | null>(null);
  const [blockReview, setBlockReview] = useState<{ value: BlockReview; key: string; port: UserBlockingPort } | null>(null);
  const live = useRef(true), blockingBusy = useRef(false);
  const current = () => live.current && presentation.isCurrent();
  useEffect(() => () => { live.current = false; owner.cancel(); }, [owner]);
  async function submit() {
    if (!reason || busy || !current()) return;
    setBusy(true); setError(null);
    const result = await owner.submit({ target, reason, explanation }, (draft, key) => presentation.reporting.submit(draft, key));
    if (result.kind === 'busy' || !current()) return;
    setBusy(false);
    if (result.kind === 'submitted') { setReceipt(result.receipt); }
    if (result.kind === 'failed') setError(result.cause instanceof ReportFailure && result.cause.kind === 'invalid' ? 'reporting.invalid' : 'reporting.failed');
  }
  async function prepareBlock() {
    const subject = receipt?.blockTarget, port = blocking;
    if (!subject || !port || port.sessionKey !== presentation.sessionKey || !current() || blockingBusy.current) return;
    blockingBusy.current = true; setBusy(true); setError(null);
    try {
      const review = await port.reviewBlock(subject.username);
      if (!current()) return;
      if (review.target.userId !== subject.userId) throw new Error('report_block_subject_changed');
      setBlockReview({ value: review, key: Crypto.randomUUID(), port });
    } catch { if (current()) setError('blocking.operationFailed'); }
    finally { blockingBusy.current = false; if (current()) setBusy(false); }
  }
  async function confirmBlock() {
    const intent = blockReview;
    if (!intent || !current() || blockingBusy.current) return;
    blockingBusy.current = true; setBusy(true); setError(null);
    try {
      const result = await intent.port.blockUser(intent.value.target.username, intent.key, intent.value.acknowledgement);
      if (!current()) return;
      if (!result.blocked || result.target.userId !== intent.value.target.userId) throw new Error('report_block_subject_changed');
      close();
    } catch { if (current()) setError('blocking.operationFailed'); }
    finally { blockingBusy.current = false; if (current()) setBusy(false); }
  }
  return <NativeSheet compact visible dismissible={!busy} onRequestClose={() => { if (!busy) close(); }} title={blockReview ? i18n.t('blocking.confirmTitle', { username: blockReview.value.target.username }) : i18n.t(receipt ? 'reporting.sent' : 'reporting.title')}
    leadingAction={receipt && !blockReview ? undefined : { label: i18n.t('common.cancel'), disabled: busy, onPress: blockReview ? () => { setBlockReview(null); setError(null); } : close }}
    trailingAction={{ label: i18n.t(blockReview ? 'blocking.blockAction' : receipt ? 'common.done' : 'reporting.submit'), disabled: busy || (!receipt && (!reason || Array.from(explanation.trim()).length > 1000)), onPress: blockReview ? () => void confirmBlock() : receipt ? close : () => void submit() }}>
    {blockReview ? <><Text style={styles.copy}>{i18n.t('blocking.confirmDescription')}</Text>{blockReview.value.sharedPaths.length > 0 && <><Text style={styles.copy}>{i18n.t('blocking.sharedPathsWarning', { username: blockReview.value.target.username, count: blockReview.value.sharedPaths.length, paths: new Intl.ListFormat(i18n.locale).format(blockReview.value.sharedPaths.map(path => path.name)) })}</Text><Text style={styles.copy}>{i18n.t('blocking.leavePathsSeparately')}</Text></>}</> : receipt ? <><Text accessibilityLiveRegion="polite" style={styles.copy}>{i18n.t('reporting.acknowledgment')}</Text>{receipt.blockTarget && <><Text style={styles.copy}>{i18n.t('reporting.blockOffer')}</Text><NativeButton label={i18n.t('blocking.blockAction')} disabled={busy || !blocking} onPress={() => void prepareBlock()} variant="secondary" /></>}</> : <>
      <View accessibilityRole="radiogroup"><SettingsSection title={i18n.t('reporting.chooseReason')}>{reportReasons.map((value, index) => <View key={value}>{index > 0 && <SettingsSeparator />}<SettingsChoiceRow label={i18n.t(`reporting.reason.${value}`)} selected={reason === value} disabled={busy} onPress={() => setReason(value)} /></View>)}</SettingsSection></View>
      <Text style={styles.copy}>{i18n.t('reporting.explanation')}</Text><ThemedTextInput accessibilityLabel={i18n.t('reporting.explanation')} multiline editable={!busy} value={explanation} onChangeText={setExplanation} style={styles.input} />
      <Text style={styles.copy}>{i18n.t('reporting.limit')}</Text>
    </>}
    {busy && <Text accessibilityLiveRegion="polite" style={styles.copy}>{i18n.t(receipt ? 'blocking.blocking' : 'reporting.submitting')}</Text>}{error && <Text accessibilityRole="alert" style={styles.copy}>{i18n.t(error)}</Text>}
  </NativeSheet>;
}
const styles = StyleSheet.create({ copy: { color: mobileTheme.colors.text, paddingVertical: mobileTheme.spacing.sm }, input: { minHeight: 88, padding: mobileTheme.spacing.md, borderRadius: mobileTheme.radii.md, backgroundColor: mobileTheme.colors.surfaceRaised, textAlignVertical: 'top' } });
