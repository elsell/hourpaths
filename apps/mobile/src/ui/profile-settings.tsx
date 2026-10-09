import { createProfileEditOperationOwner, ProfileEditFailure, type EditableProfile, type ProfileEditingRepository } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { SettingsSection } from './settings-list';
import { ThemedText as Text, ThemedTextInput as TextInput, StatusBanner } from './primitives';
import { NativeButton } from './native-button';
import { mobileTheme } from './tokens';

export function ProfileSettings({ repository, operationId, isCurrent, i18n }: { repository: ProfileEditingRepository; operationId(): string; isCurrent(): boolean; i18n: Translator }) {
  const [saved, setSaved] = useState<EditableProfile | null>(null), [draft, setDraft] = useState<EditableProfile | null>(null);
  const [busy, setBusy] = useState(false), [loadFailed, setLoadFailed] = useState(false);
  const [error, setError] = useState<ProfileEditFailure | null>(null), [notice, setNotice] = useState<'saved' | 'review' | null>(null);
  const [owner] = useState(() => createProfileEditOperationOwner(operationId));
  const alive = useRef(true), admission = useRef(false), current = useRef(isCurrent), port = useRef(repository);
  current.current = isCurrent; port.current = repository;
  const active = () => alive.current && current.current();
  async function load(preserve = false) {
    if (admission.current) return;
    admission.current = true; setBusy(true); setLoadFailed(false);
    try {
      const value = await port.current.read();
      if (!active()) return;
      owner.cancel(); setSaved(value); setDraft(previous => preserve && previous ? { ...previous, revision: value.revision } : value);
      setError(null); setNotice(preserve ? 'review' : null);
    } catch { if (active()) setLoadFailed(true); }
    finally { admission.current = false; if (active()) setBusy(false); }
  }
  useEffect(() => { alive.current = true; void load(); return () => { alive.current = false; owner.cancel(); }; }, [owner]);
  async function save() {
    if (!draft || admission.current) return;
    admission.current = true; setBusy(true); setError(null); setNotice(null);
    const result = await owner.submit(draft, (value, key) => port.current.save(value, key));
    admission.current = false;
    if (!active()) return;
    setBusy(false);
    if (result.kind === 'applied') { setSaved(result.profile); setDraft(result.profile); setNotice('saved'); }
    else if (result.kind === 'failed') setError(result.cause instanceof ProfileEditFailure ? result.cause : new ProfileEditFailure());
  }
  const dirty = !!draft && !!saved && (draft.displayName !== saved.displayName || draft.username !== saved.username || draft.description !== saved.description);
  return <SettingsSection title={i18n.t('profile.edit.heading')} footer={i18n.t('profile.edit.hint')}><View style={styles.form}>
    {draft && <>{(['displayName', 'username', 'description'] as const).map(field => {
      const label = i18n.t(field === 'displayName' ? 'profile.edit.name' : field === 'username' ? 'profile.edit.username' : 'profile.edit.description');
      return <View key={field} style={styles.field}><Text>{label}</Text><TextInput accessibilityLabel={label} value={draft[field]} editable={!busy} autoCapitalize={field === 'username' ? 'none' : 'sentences'} autoCorrect={field !== 'username'} multiline={field === 'description'} onChangeText={value => { setDraft(previous => previous ? { ...previous, [field]: value } : previous); setNotice(null); if (error?.kind !== 'conflict') setError(null); }} /></View>;
    })}<NativeButton label={i18n.t('common.save')} fullWidth busy={busy} disabled={!dirty || error?.kind === 'conflict'} onPress={() => void save()} />
      {dirty && <NativeButton label={i18n.t('common.cancel')} variant="quiet" disabled={busy} onPress={() => { setDraft(saved); setNotice(null); if (error?.kind !== 'conflict') setError(null); }} />}</>}
    {!draft && busy && <StatusBanner text={i18n.t('common.loading')} />}
    {loadFailed && <StatusBanner tone="error" text={i18n.t('profile.edit.loadFailed')} actionLabel={i18n.t('common.retry')} onAction={() => void load(!!draft)} />}
    {error && <StatusBanner tone="error" text={i18n.t(`profile.edit.${error.kind}`)} actionLabel={error.kind === 'conflict' ? i18n.t('profile.edit.reload') : undefined} onAction={error.kind === 'conflict' && !busy ? () => void load(true) : undefined} />}
    {notice && <Text accessibilityLiveRegion="polite">{i18n.t(notice === 'saved' ? 'studio.settings.saved' : 'profile.edit.review')}</Text>}
  </View></SettingsSection>;
}
const styles = StyleSheet.create({ form: { padding: mobileTheme.spacing.md, gap: mobileTheme.spacing.md }, field: { gap: mobileTheme.spacing.xs } });
