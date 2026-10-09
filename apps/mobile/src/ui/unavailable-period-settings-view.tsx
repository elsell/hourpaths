import { createUnavailablePeriodOwner, UnavailablePeriodFailure, type UnavailablePeriodPreference, type UnavailablePeriodRepository } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import * as Crypto from 'expo-crypto';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeTimeField } from './native-time-field';
import { NativeButton } from './native-button';
import { SettingsSwitchRow, SettingsSection, SettingsSeparator, SettingsShell } from './settings-list';
import { ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

export function UnavailablePeriodSettingsView({ i18n, repository }: { i18n: Translator; repository: UnavailablePeriodRepository }) {
  const repo = useRef(repository); repo.current = repository;
  const current = useRef(true), ticket = useRef(0);
  const [initial, setInitial] = useState<UnavailablePeriodPreference>(), [failed, setFailed] = useState(false);
  async function load() {
    const sequence = ++ticket.current; setFailed(false);
    try { const value = await repo.current.read(); if (current.current && sequence === ticket.current) setInitial(value); }
    catch { if (current.current && sequence === ticket.current) setFailed(true); }
  }
  useEffect(() => { current.current = true; void load(); return () => { current.current = false; ticket.current++; }; }, []);
  if (initial) return <UnavailablePeriodForm key={initial.userId} i18n={i18n} repository={repository} initial={initial} />;
  return <SettingsShell>{failed ? <><NativeContentUnavailable title={i18n.t('settings.quietHours.heading')} description={i18n.t('settings.quietHours.loadFailed')} systemImage="calendar" /><NativeButton label={i18n.t('common.retry')} variant="quiet" onPress={() => void load()} /></> : <ActivityIndicator accessibilityLabel={i18n.t('common.loading')} color={mobileTheme.colors.accent} />}</SettingsShell>;
}
function UnavailablePeriodForm({ i18n, repository, initial }: { i18n: Translator; repository: UnavailablePeriodRepository; initial: UnavailablePeriodPreference }) {
  const repo = useRef(repository); repo.current = repository;
  const current = useRef(true), admitted = useRef(false);
  const [saved, setSaved] = useState(initial), [draft, setDraft] = useState(initial);
  const [busy, setBusy] = useState(false), [error, setError] = useState<'conflict' | 'failed' | null>(null), [success, setSuccess] = useState(false);
  const [owner] = useState(() => createUnavailablePeriodOwner(initial.userId, Crypto.randomUUID));
  useEffect(() => { current.current = true; return () => { current.current = false; owner.cancel(); }; }, [owner]);
  const dirty = draft.enabled !== saved.enabled || draft.startMinute !== saved.startMinute || draft.endMinute !== saved.endMinute;
  async function refresh() {
    if (admitted.current) return; admitted.current = true; setBusy(true);
    try { const value = await repo.current.read(); if (current.current) { owner.cancel(); setSaved(value); setDraft(value); setError(null); setSuccess(false); } }
    catch { if (current.current) setError('failed'); }
    finally { admitted.current = false; if (current.current) setBusy(false); }
  }
  async function save() {
    if (admitted.current || !dirty || error === 'conflict') return; admitted.current = true; setBusy(true); setError(null); setSuccess(false);
    const result = await owner.submit({ userId: saved.userId, enabled: draft.enabled, startMinute: draft.startMinute, endMinute: draft.endMinute, expectedRevision: saved.revision, reviewedTimeZone: saved.timeZone }, (value, key) => repo.current.save(value, key));
    if (current.current) {
      if (result.kind === 'applied') {
        setSaved(result.preference); setDraft(result.preference); setSuccess(true);
        try { const latest = await repo.current.read(); if (current.current) { setSaved(latest); setDraft(latest); } } catch { /* The acknowledged save remains available; reopening refreshes it. */ }
      } else if (result.kind === 'failed') setError(result.cause instanceof UnavailablePeriodFailure && result.cause.kind === 'conflict' ? 'conflict' : 'failed');
      if (current.current) setBusy(false);
    }
    admitted.current = false;
  }
  return <SettingsShell><SettingsSection footer={i18n.t('settings.quietHours.explanation')}>
    <SettingsSwitchRow accessibilityLabel={i18n.t('settings.quietHours.enabled')} label={i18n.t('settings.quietHours.enabled')} value={draft.enabled} valueLabel={i18n.t(draft.enabled ? 'settings.quietHours.on' : 'settings.quietHours.off')} disabled={busy} onValueChange={enabled => { setDraft({ ...draft, enabled }); setSuccess(false); }} />
    {draft.enabled && <><SettingsSeparator /><NativeTimeField i18n={i18n} label={i18n.t('settings.quietHours.start')} minute={draft.startMinute} disabled={busy} onChange={startMinute => { setDraft({ ...draft, startMinute }); setSuccess(false); }} /><SettingsSeparator /><NativeTimeField i18n={i18n} label={i18n.t('settings.quietHours.end')} minute={draft.endMinute} disabled={busy} onChange={endMinute => { setDraft({ ...draft, endMinute }); setSuccess(false); }} /></>}
  </SettingsSection>
    <Text>{i18n.t('settings.quietHours.zone', { zone: saved.timeZone })}</Text>
    {error && <Text accessibilityRole="alert">{i18n.t(error === 'conflict' ? 'settings.quietHours.conflict' : 'settings.quietHours.saveFailed')}</Text>}
    {error === 'conflict' && <NativeButton label={i18n.t('common.refresh')} variant="quiet" disabled={busy} onPress={() => void refresh()} />}
    {success && <Text accessibilityLiveRegion="polite">{i18n.t('settings.quietHours.saved')}</Text>}
    <NativeButton label={i18n.t(busy ? 'settings.quietHours.saving' : 'common.save')} busy={busy} disabled={!dirty || error === 'conflict'} onPress={() => void save()} />
    <NativeButton label={i18n.t('common.cancel')} variant="quiet" disabled={!dirty || busy} onPress={() => { owner.cancel(); setDraft(saved); setSuccess(false); }} />
  </SettingsShell>;
}
