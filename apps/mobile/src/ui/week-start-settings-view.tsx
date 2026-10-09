import { createWeekStartOperationOwner, WeekStartFailure, type WeekStartPreference, type WeekStartPreferenceRepository } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import * as Crypto from 'expo-crypto';
import { Fragment, useEffect, useRef, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { NativeContentUnavailable } from './native-content-unavailable';
import { NativeButton } from './native-button';
import { SettingsChoiceRow, SettingsSection, SettingsSeparator, SettingsShell } from './settings-list';
import { ThemedText as Text } from './primitives';
import { mobileTheme } from './tokens';

export function WeekStartSettingsView({ i18n, repository }: { i18n: Translator; repository: WeekStartPreferenceRepository }) {
  const repo = useRef(repository); repo.current = repository;
  const current = useRef(true), ticket = useRef(0);
  const [initial, setInitial] = useState<WeekStartPreference>(), [failed, setFailed] = useState(false);
  async function load() {
    const sequence = ++ticket.current; setFailed(false);
    try { const value = await repo.current.read(); if (current.current && sequence === ticket.current) setInitial(value); }
    catch { if (current.current && sequence === ticket.current) setFailed(true); }
  }
  useEffect(() => { current.current = true; void load(); return () => { current.current = false; ticket.current++; }; }, []);
  if (initial) return <WeekStartForm key={initial.userId} i18n={i18n} repository={repository} initial={initial} />;
  return <SettingsShell>{failed ? <><NativeContentUnavailable title={i18n.t('settings.weekStart.heading')} description={i18n.t('settings.weekStart.loadFailed')} systemImage="calendar" /><NativeButton label={i18n.t('common.retry')} variant="quiet" onPress={() => void load()} /></> : <ActivityIndicator accessibilityLabel={i18n.t('common.loading')} color={mobileTheme.colors.accent} />}</SettingsShell>;
}
function WeekStartForm({ i18n, repository, initial }: { i18n: Translator; repository: WeekStartPreferenceRepository; initial: WeekStartPreference }) {
  const repo = useRef(repository); repo.current = repository;
  const current = useRef(true), admitted = useRef(false);
  const [saved, setSaved] = useState(initial), [draft, setDraft] = useState(initial.firstDayOfWeek);
  const [busy, setBusy] = useState(false), [error, setError] = useState<'conflict' | 'failed' | null>(null), [success, setSuccess] = useState(false);
  const [owner] = useState(() => createWeekStartOperationOwner(initial.userId, Crypto.randomUUID));
  useEffect(() => { current.current = true; return () => { current.current = false; owner.cancel(); }; }, [owner]);
  const weekdayLabel = (day: number) => i18n.date(Date.UTC(2026, 0, 4 + day), {
    weekday: 'long', timeZone: 'UTC',
  });
  const dirty = draft !== saved.firstDayOfWeek;
  async function refresh() {
    if (admitted.current) return; admitted.current = true; setBusy(true);
    try { const value = await repo.current.read(); if (current.current) { owner.cancel(); setSaved(value); setDraft(value.firstDayOfWeek); setError(null); setSuccess(false); } }
    catch { if (current.current) setError('failed'); }
    finally { admitted.current = false; if (current.current) setBusy(false); }
  }
  async function save() {
    if (admitted.current || !dirty || error === 'conflict') return; admitted.current = true; setBusy(true); setError(null); setSuccess(false);
    const result = await owner.submit({ userId: saved.userId, reviewedFirstDayOfWeek: saved.firstDayOfWeek, proposedFirstDayOfWeek: draft }, (value, key) => repo.current.save(value, key));
    if (current.current) {
      if (result.kind === 'applied') {
        setSaved(result.preference); setDraft(result.preference.firstDayOfWeek); setSuccess(true);
        try { const latest = await repo.current.read(); if (current.current) { setSaved(latest); setDraft(latest.firstDayOfWeek); } } catch { /* The acknowledged save remains available; reopening refreshes it. */ }
      } else if (result.kind === 'failed') setError(result.cause instanceof WeekStartFailure && result.cause.kind === 'conflict' ? 'conflict' : 'failed');
      if (current.current) setBusy(false);
    }
    admitted.current = false;
  }
  return <SettingsShell><SettingsSection footer={i18n.t('settings.weekStart.explanation')}>
    {Array.from({ length: 7 }, (_, index) => index + 1).map((day, index) => <Fragment key={day}>{index > 0 && <SettingsSeparator />}<SettingsChoiceRow label={weekdayLabel(day)} selected={draft === day} disabled={busy} onPress={() => { setDraft(day); setSuccess(false); }} /></Fragment>)}
  </SettingsSection>
    {error && <Text accessibilityRole="alert">{i18n.t(error === 'conflict' ? 'settings.weekStart.conflict' : 'settings.weekStart.saveFailed')}</Text>}
    {error === 'conflict' && <NativeButton label={i18n.t('common.refresh')} variant="quiet" disabled={busy} onPress={() => void refresh()} />}
    {success && <Text accessibilityLiveRegion="polite">{i18n.t('settings.weekStart.saved')}</Text>}
    <NativeButton label={i18n.t(busy ? 'settings.weekStart.saving' : 'common.save')} busy={busy} disabled={!dirty || error === 'conflict'} onPress={() => void save()} />
    <NativeButton label={i18n.t('common.cancel')} variant="quiet" disabled={!dirty || busy} onPress={() => { owner.cancel(); setDraft(saved.firstDayOfWeek); setSuccess(false); }} />
  </SettingsShell>;
}
