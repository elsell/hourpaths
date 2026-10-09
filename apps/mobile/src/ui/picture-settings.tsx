import { centerPictureCrop, createPictureOperationOwner, PictureFailure, type ProfilePicture, type ProfilePictureRepository, type PicturePreview } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { useEffect, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import { SettingsSection } from './settings-list';
import { ThemedText as Text, StatusBanner } from './primitives';
import { NativeButton } from './native-button';
import { SocialProfileAvatar } from './social-profile-avatar';
import { mobileTheme } from './tokens';

export function PictureSettings({ repository, pick, operationId, isCurrent, i18n }: {
  repository: ProfilePictureRepository; pick(): Promise<string | null>; operationId(): string; isCurrent(): boolean; i18n: Translator;
}) {
  const [saved, setSaved] = useState<ProfilePicture | null>(null);
  const [draft, setDraft] = useState<{ image: string; preview: PicturePreview } | null>(null);
  const [busy, setBusy] = useState(false), [loadFailed, setLoadFailed] = useState(false);
  const [error, setError] = useState<PictureFailure | null>(null), [notice, setNotice] = useState<'saved' | 'removed' | 'review' | null>(null);
  const [owner] = useState(() => createPictureOperationOwner(operationId));
  const alive = useRef(true), admission = useRef(false), current = useRef(isCurrent), port = useRef(repository);
  current.current = isCurrent; port.current = repository;
  const active = () => alive.current && current.current();
  const fail = (cause: unknown) => setError(cause instanceof PictureFailure ? cause : new PictureFailure());
  async function load(preserve = false) {
    if (admission.current) return;
    admission.current = true; setBusy(true); setLoadFailed(false);
    try {
      const value = await port.current.read();
      if (!active()) return;
      owner.cancel(); setSaved(value); setError(null); setNotice(preserve ? 'review' : null);
    } catch { if (active()) setLoadFailed(true); }
    finally { admission.current = false; if (active()) setBusy(false); }
  }
  useEffect(() => { alive.current = true; void load(); return () => { alive.current = false; owner.cancel(); }; }, [owner]);
  async function choose() {
    if (admission.current || !saved) return;
    admission.current = true; setBusy(true); setError(null); setNotice(null); owner.cancel();
    try {
      const image = await pick();
      if (!active() || image === null) return;
      const result = await owner.preview(image, bytes => port.current.preview(bytes));
      if (!active()) return;
      if (result.kind === 'preview') setDraft({ image, preview: result.preview });
      else if (result.kind === 'failed') fail(result.cause);
    } catch (cause) { if (active()) fail(cause); }
    finally { admission.current = false; if (active()) setBusy(false); }
  }
  async function save(remove = false) {
    if (admission.current || !saved || (!remove && !draft)) return;
    admission.current = true; setBusy(true); setError(null); setNotice(null);
    const result = await owner.submit({ userId: saved.userId, revision: saved.revision, image: remove ? '' : draft!.image, crop: remove ? undefined : centerPictureCrop(draft!.preview), remove }, (value, key) => port.current.save(value, key));
    admission.current = false;
    if (!active()) return;
    setBusy(false);
    if (result.kind === 'applied') { setSaved(result.picture); setDraft(null); setNotice(remove ? 'removed' : 'saved'); }
    else if (result.kind === 'failed') fail(result.cause);
  }
  return <SettingsSection title={i18n.t('picture.heading')} footer={i18n.t('picture.hint')}><View style={styles.form}>
    {saved && <><View style={styles.preview}><SocialProfileAvatar size={112} accessibilityLabel={i18n.t('picture.preview')} profilePictureURL={draft ? `data:image/jpeg;base64,${draft.preview.image}` : saved.url || undefined} /></View>
      <NativeButton label={i18n.t(saved.url || draft ? 'picture.change' : 'picture.choose')} variant="quiet" disabled={busy} onPress={() => void choose()} />
      {draft ? <><NativeButton label={i18n.t('common.save')} fullWidth busy={busy} disabled={error?.kind === 'conflict'} onPress={() => void save()} /><NativeButton label={i18n.t('common.cancel')} variant="quiet" disabled={busy} onPress={() => { owner.cancel(); setDraft(null); setError(null); setNotice(null); }} /></> : saved.url ? <NativeButton label={i18n.t('picture.remove')} variant="quiet" busy={busy} onPress={() => void save(true)} /> : null}</>}
    {!saved && busy && <StatusBanner text={i18n.t('common.loading')} />}
    {loadFailed && <StatusBanner tone="error" text={i18n.t('picture.unavailable')} actionLabel={i18n.t('common.retry')} onAction={() => void load(!!draft)} />}
    {error && <StatusBanner tone="error" text={i18n.t(`picture.${error.kind}`)} actionLabel={error.kind === 'conflict' ? i18n.t('picture.reload') : undefined} onAction={error.kind === 'conflict' && !busy ? () => void load(true) : undefined} />}
    {notice && <Text accessibilityLiveRegion="polite">{i18n.t(`picture.${notice}`)}</Text>}
  </View></SettingsSection>;
}
const styles = StyleSheet.create({ form: { padding: mobileTheme.spacing.md, gap: mobileTheme.spacing.md }, preview: { alignItems: 'center' } });
