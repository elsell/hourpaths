import { appealAvailable, createAppealSubmissionOwner, EnforcementFailure, type EnforcementNotice, type EnforcementRepository } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import * as Crypto from 'expo-crypto';
import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { SettingsSection, SettingsShell } from './settings-list';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeButton } from './native-button';
import { ThemedText as Text, ThemedTextInput } from './primitives';
import { mobileTheme } from './tokens';

export function EnforcementSettingsView({ i18n, repository }: { i18n: Translator; repository: EnforcementRepository }) {
  const repo = useRef(repository); repo.current = repository;
  const live = useRef(true), ticket = useRef(0), admitted = useRef(false);
  const [items, setItems] = useState<EnforcementNotice[]>(), [next, setNext] = useState<string>();
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  async function load(cursor?: string) {
    if (admitted.current) return; admitted.current = true;
    const sequence = ++ticket.current; setBusy(true); setFailed(false);
    try {
      const page = await repo.current.list(cursor);
      if (live.current && ticket.current === sequence) { setItems(previous => cursor ? [...(previous ?? []), ...page.items.filter(value => !previous?.some(old => old.id === value.id))] : page.items); setNext(page.nextCursor); }
    } catch { if (live.current && ticket.current === sequence) setFailed(true); }
    finally { admitted.current = false; if (live.current && ticket.current === sequence) setBusy(false); }
  }
  useEffect(() => { live.current = true; void load(); return () => { live.current = false; ticket.current++; }; }, []);
  return <SettingsShell>
    {!items && busy && <ActivityIndicator accessibilityLabel={i18n.t('common.loading')} color={mobileTheme.colors.accent} />}
    {items?.length === 0 && <NativeContentUnavailable title={i18n.t('enforcement.empty')} description={i18n.t('enforcement.emptyDescription')} systemImage="checkmark.circle" />}
    {items?.map(notice => <Notice key={notice.id} notice={notice} i18n={i18n} repository={repository} update={value => setItems(previous => previous?.map(old => old.id === value.id ? value : old))} />)}
    {failed && <Text accessibilityRole="alert">{i18n.t('enforcement.loadFailed')}</Text>}
    {(items || failed) && <NativeButton label={i18n.t(failed ? 'common.retry' : 'common.refresh')} busy={busy} variant="quiet" onPress={() => void load()} />}
    {next && <NativeButton label={i18n.t('common.loadMore')} disabled={busy} variant="quiet" onPress={() => void load(next)} />}
  </SettingsShell>;
}
function Notice({ notice, i18n, repository, update }: { notice: EnforcementNotice; i18n: Translator; repository: EnforcementRepository; update(value: EnforcementNotice): void }) {
  const repo = useRef(repository); repo.current = repository;
  const live = useRef(true), admitted = useRef(false);
  const [owner] = useState(() => createAppealSubmissionOwner(Crypto.randomUUID));
  const [explanation, setExplanation] = useState(''), [editing, setEditing] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState<'failed' | 'conflict' | null>(null);
  useEffect(() => { live.current = true; return () => { live.current = false; owner.cancel(); }; }, [owner]);
  async function submit() {
    if (admitted.current) return; admitted.current = true; setBusy(true); setError(null);
    const result = await owner.submit(notice.id, explanation, (id, text, key) => repo.current.appeal(id, text, key));
    if (live.current) {
      if (result.kind === 'submitted') { update({ ...notice, appeal: result.appeal }); setEditing(false); setExplanation(''); }
      if (result.kind === 'failed') setError(result.cause instanceof EnforcementFailure && result.cause.kind === 'conflict' ? 'conflict' : 'failed');
      setBusy(false);
    }
    admitted.current = false;
  }
  const date = (value: string) => i18n.date(Date.parse(value), { dateStyle: 'medium', timeStyle: 'short' });
  return <SettingsSection title={i18n.t(`enforcement.action.${notice.action}`)}><View style={styles.content}>
    {notice.affectedComment && <Text>{i18n.t('enforcement.affectedComment', { date: date(notice.affectedComment.createdAt), id: notice.affectedComment.id })}</Text>}
    <Text>{date(notice.issuedAt)}</Text><Text>{notice.policyReason}</Text>
    {notice.until && <Text>{i18n.t('enforcement.until', { date: date(notice.until) })}</Text>}
    {notice.appeal ? <>
      <Text accessibilityLiveRegion="polite">{i18n.t(notice.appeal.outcome ? `enforcement.${notice.appeal.outcome}` : 'enforcement.pending')}</Text>
      {notice.appeal.explanation !== '' && <Text>{notice.appeal.explanation}</Text>}
      {notice.appeal.decisionReason && <Text>{notice.appeal.decisionReason}</Text>}
    </> : <>
      <Text>{i18n.t('enforcement.appealDeadline', { date: date(notice.appealDeadline) })}</Text>
      {appealAvailable(notice, Date.now()) ? editing ? <>
        <Text>{i18n.t('enforcement.explanation')}</Text>
        <ThemedTextInput accessibilityLabel={i18n.t('enforcement.explanation')} multiline editable={!busy} value={explanation} onChangeText={setExplanation} style={styles.input} />
        <NativeButton label={i18n.t(busy ? 'enforcement.submitting' : 'enforcement.submit')} busy={busy} disabled={error === 'conflict'} onPress={() => void submit()} />
        <NativeButton label={i18n.t('common.cancel')} variant="quiet" disabled={busy} onPress={() => { setEditing(false); setError(null); }} />
      </> : <NativeButton label={i18n.t('enforcement.appeal')} variant="secondary" onPress={() => setEditing(true)} /> : <Text>{i18n.t('enforcement.closed')}</Text>}
    </>}
    {error && <Text accessibilityRole="alert">{i18n.t(`enforcement.${error}`)}</Text>}
  </View></SettingsSection>;
}
const styles = StyleSheet.create({ content: { padding: mobileTheme.spacing.md, gap: mobileTheme.spacing.md }, input: { minHeight: 100, padding: mobileTheme.spacing.md, backgroundColor: mobileTheme.colors.surfaceRaised, borderRadius: mobileTheme.radii.md, textAlignVertical: 'top' } });
