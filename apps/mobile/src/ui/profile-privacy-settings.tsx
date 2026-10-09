import { createProfilePrivacyOperationOwner, ProfilePrivacyFailure, type ProfilePrivacy, type ProfilePrivacyRepository, type ProfileVisibility } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { SettingsSection, SettingsValueRow } from './settings-list';
import { ThemedText as Text, StatusBanner } from './primitives';
import { NativeButton } from './native-button';
import { mobileTheme } from './tokens';

export function ProfilePrivacySettings({ repository, operationId, isCurrent, confirm, i18n }: { repository: ProfilePrivacyRepository; operationId(): string; isCurrent(): boolean; confirm(visibility: ProfileVisibility, accepted: () => void): void; i18n: Translator }) {
  const [profile, setProfile] = useState<ProfilePrivacy | null>(null), [busy, setBusy] = useState(false);
  const [error, setError] = useState<ProfilePrivacyFailure | null>(null), [loadFailed, setLoadFailed] = useState(false), [saved, setSaved] = useState(false);
  const [mutation] = useState(() => createProfilePrivacyOperationOwner(operationId));
  const alive = useRef(true), admitted = useRef(false), current = useRef(isCurrent), port = useRef(repository);
  current.current = isCurrent; port.current = repository;
  const active = () => alive.current && current.current();
  async function load() {
    if (admitted.current) return;
    admitted.current = true; setBusy(true); setLoadFailed(false);
    try {
      const value = await port.current.read();
      if (!active()) return;
      mutation.cancel(); setProfile(value); setError(null); setSaved(false);
    } catch { if (active()) setLoadFailed(true); }
    finally { admitted.current = false; if (active()) setBusy(false); }
  }
  useEffect(() => { alive.current = true; void load(); return () => { alive.current = false; mutation.cancel(); }; }, [mutation]);
  async function save(reviewed: ProfilePrivacy, proposed: ProfileVisibility) {
    if (!active() || admitted.current) return;
    admitted.current = true; setBusy(true); setError(null); setSaved(false);
    const result = await mutation.submit(reviewed, proposed, (value, visibility, key) => port.current.save(value, visibility, key));
    admitted.current = false;
    if (!active()) return;
    setBusy(false);
    if (result.kind === 'applied') { setProfile(result.profile); setSaved(true); }
    else if (result.kind === 'failed') setError(result.cause instanceof ProfilePrivacyFailure ? result.cause : new ProfilePrivacyFailure());
  }
  function review() {
    if (!profile || admitted.current) return;
    const frozen = { ...profile }, proposed = profile.visibility === 'private' ? 'public' : 'private';
    confirm(proposed, () => void save(frozen, proposed));
  }
  return <SettingsSection title={i18n.t('profile.privacy.heading')} footer={i18n.t('profile.privacy.hint')}>
    {profile && <SettingsValueRow label={i18n.t('profile.privacy.heading')} value={i18n.t(profile.visibility === 'private' ? 'profile.privacy.private' : 'profile.privacy.public')} />}
    <View style={styles.content}>
      {profile && <NativeButton label={i18n.t(profile.visibility === 'private' ? 'profile.privacy.makePublic' : 'profile.privacy.makePrivate')} busy={busy} disabled={error?.kind === 'conflict'} onPress={review} />}
      {!profile && busy && <StatusBanner text={i18n.t('common.loading')} />}
      {loadFailed && <StatusBanner tone="error" text={i18n.t('profile.privacy.loadFailed')} actionLabel={i18n.t('common.retry')} onAction={() => void load()} />}
      {error && <StatusBanner tone="error" text={i18n.t(`profile.privacy.${error.kind}`)} actionLabel={i18n.t('common.refresh')} onAction={!busy ? () => void load() : undefined} />}
      {saved && <Text accessibilityLiveRegion="polite">{i18n.t('studio.settings.saved')}</Text>}
    </View>
  </SettingsSection>;
}
const styles = StyleSheet.create({ content: { padding: mobileTheme.spacing.md, gap: mobileTheme.spacing.md } });
