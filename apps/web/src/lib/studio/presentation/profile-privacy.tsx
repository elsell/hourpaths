import { createProfilePrivacyOperationOwner, ProfilePrivacyFailure, type ProfilePrivacy, type ProfileVisibility } from '@hourpaths/client-core';
import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';
import { ConfirmationDialog } from './confirmation-dialog';

export function ProfilePrivacySettings({ owner, dependencies: d }: { owner: string; dependencies: StudioDependencies }) {
  const cache = useQueryClient(), operation = useOwnedOperation(d);
  const query = useQuery({ queryKey: [d.accountScope, 'preferences', 'profilePrivacy'], queryFn: ({ signal }) => d.preferences.profilePrivacy(owner, signal) });
  const [review, setReview] = useState<{ profile: ProfilePrivacy; proposed: ProfileVisibility } | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState<ProfilePrivacyFailure | null>(null), [saved, setSaved] = useState(false);
  const [mutation] = useState(() => createProfilePrivacyOperationOwner(d.operationId));
  const admitted = useRef(false);
  useEffect(() => () => mutation.cancel(), [mutation]);
  async function save() {
    if (!review || admitted.current) return;
    admitted.current = true; setBusy(true); setError(null); setSaved(false);
    const result = await mutation.submit(review.profile, review.proposed, (value, proposed, key) => operation.run(signal => d.preferences.saveProfilePrivacy(value, proposed, key, signal)));
    admitted.current = false;
    if (!operation.active()) return;
    setBusy(false);
    if (result.kind === 'applied') {
      cache.setQueryData([d.accountScope, 'preferences', 'profilePrivacy'], result.profile);
      setReview(null); setSaved(true);
      void cache.invalidateQueries({ queryKey: [d.accountScope] });
    } else if (result.kind === 'failed') { setError(result.cause instanceof ProfilePrivacyFailure ? result.cause : new ProfilePrivacyFailure()); setReview(null); }
  }
  async function reload() {
    if (admitted.current) return;
    admitted.current = true; setBusy(true);
    try {
      const current = await operation.run(signal => d.preferences.profilePrivacy(owner, signal));
      if (!operation.active()) return;
      mutation.cancel(); setReview(null); setError(null); setSaved(false);
      cache.setQueryData([d.accountScope, 'preferences', 'profilePrivacy'], current);
    } catch { if (operation.active()) setError(new ProfilePrivacyFailure()); }
    finally { admitted.current = false; if (operation.active()) setBusy(false); }
  }
  return <section className="studio-settings-card"><h3>{d.i18n.t('profile.privacy.heading')}</h3><p>{d.i18n.t('profile.privacy.hint')}</p>
    {query.data ? <><p>{d.i18n.t(query.data.visibility === 'private' ? 'profile.privacy.private' : 'profile.privacy.public')}</p><button disabled={busy || error?.kind === 'conflict'} onClick={() => { setSaved(false); setReview({ profile: { ...query.data! }, proposed: query.data!.visibility === 'private' ? 'public' : 'private' }); }}>{d.i18n.t(query.data.visibility === 'private' ? 'profile.privacy.makePublic' : 'profile.privacy.makePrivate')}</button></> : query.isError ? <div role="alert"><p>{d.i18n.t('profile.privacy.loadFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <p role="status">{d.i18n.t('common.loading')}</p>}
    {error && <div role="alert"><p>{d.i18n.t(`profile.privacy.${error.kind}`)}</p><button disabled={busy} onClick={() => void reload()}>{d.i18n.t('common.refresh')}</button></div>}
    {saved && <p role="status">{d.i18n.t('studio.settings.saved')}</p>}
    {review && <ConfirmationDialog title={d.i18n.t(review.proposed === 'private' ? 'profile.privacy.makePrivate' : 'profile.privacy.makePublic')} busy={busy} cancelLabel={d.i18n.t('common.cancel')} confirmLabel={d.i18n.t('profile.privacy.confirm')} cancel={() => setReview(null)} confirm={() => void save()}><p>{d.i18n.t(review.proposed === 'private' ? 'profile.privacy.privateWarning' : 'profile.privacy.publicWarning')}</p></ConfirmationDialog>}
  </section>;
}
