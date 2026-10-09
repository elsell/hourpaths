import { createProfileEditOperationOwner, ProfileEditFailure, type EditableProfile } from '@hourpaths/client-core';
import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';
import { UnsavedChanges } from './unsaved-changes';

export function ProfileSettings({ owner, dependencies: d }: { owner: string; dependencies: StudioDependencies }) {
  const query = useQuery({ queryKey: [d.accountScope, 'preferences', 'profile'], queryFn: ({ signal }) => d.preferences.editableProfile(owner, signal) });
  return <section className="studio-settings-card"><h3>{d.i18n.t('profile.edit.heading')}</h3><p>{d.i18n.t('profile.edit.hint')}</p>
    {query.data ? <ProfileForm initial={query.data} dependencies={d} /> : query.isError ? <div role="alert"><p>{d.i18n.t('profile.edit.loadFailed')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <p role="status">{d.i18n.t('common.loading')}</p>}
  </section>;
}
function ProfileForm({ initial, dependencies: d }: { initial: EditableProfile; dependencies: StudioDependencies }) {
  const [saved, setSaved] = useState(initial), [draft, setDraft] = useState(initial);
  const [busy, setBusy] = useState(false), [error, setError] = useState<ProfileEditFailure | null>(null);
  const [notice, setNotice] = useState<'saved' | 'review' | null>(null);
  const [owner] = useState(() => createProfileEditOperationOwner(d.operationId));
  const operation = useOwnedOperation(d), cache = useQueryClient();
  useEffect(() => () => owner.cancel(), [owner]);
  const dirty = draft.username !== saved.username || draft.displayName !== saved.displayName || draft.description !== saved.description;
  async function save() {
    if (busy) return;
    setBusy(true); setError(null); setNotice(null);
    const result = await owner.submit(draft, (value, key) => operation.run(signal => d.preferences.saveProfile(value, key, signal)));
    if (!operation.active()) return;
    setBusy(false);
    if (result.kind === 'applied') {
      setSaved(result.profile); setDraft(result.profile); setNotice('saved');
      cache.setQueryData([d.accountScope, 'preferences', 'profile'], result.profile);
      void cache.invalidateQueries({ queryKey: [d.accountScope, 'preferences', 'identity'] });
      void cache.invalidateQueries({ queryKey: [d.accountScope, 'social'] });
    } else if (result.kind === 'failed') setError(result.cause instanceof ProfileEditFailure ? result.cause : new ProfileEditFailure());
  }
  async function reload() {
    if (busy) return;
    setBusy(true);
    try {
      const latest = await operation.run(signal => d.preferences.editableProfile(saved.userId, signal));
      if (!operation.active()) return;
      owner.cancel(); setSaved(latest); setDraft(value => ({ ...value, revision: latest.revision })); setError(null); setNotice('review');
    } catch { /* Keep the conflict and draft visible until a fresh version is available. */ }
    finally { if (operation.active()) setBusy(false); }
  }
  return <form className="studio-settings-form" onSubmit={event => { event.preventDefault(); void save(); }}>
    <UnsavedChanges dirty={dirty || busy} i18n={d.i18n} />
    <fieldset disabled={busy}>
      {(['displayName', 'username', 'description'] as const).map(field => {
        const label = d.i18n.t(field === 'displayName' ? 'profile.edit.name' : field === 'username' ? 'profile.edit.username' : 'profile.edit.description');
        const change = (value: string) => { setDraft(current => ({ ...current, [field]: value })); setNotice(null); if (error?.kind !== 'conflict') setError(null); };
        return <label key={field}>{label}{field === 'description' ? <textarea value={draft[field]} rows={3} onChange={event => change(event.target.value)} /> : <input value={draft[field]} autoCapitalize={field === 'username' ? 'none' : 'words'} autoCorrect={field === 'username' ? 'off' : undefined} onChange={event => change(event.target.value)} />}</label>;
      })}
      <div className="studio-settings-actions"><button type="button" disabled={!dirty} onClick={() => { setDraft(saved); setNotice(null); if (error?.kind !== 'conflict') setError(null); }}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" disabled={!dirty || error?.kind === 'conflict'}>{d.i18n.t('common.save')}</button></div>
    </fieldset>
    {busy && <p role="status">{d.i18n.t('studio.settings.saving')}</p>}
    {error && <p role="alert">{d.i18n.t(`profile.edit.${error.kind}`)}</p>}
    {error?.kind === 'conflict' && <button type="button" disabled={busy} onClick={() => void reload()}>{d.i18n.t('profile.edit.reload')}</button>}
    {notice && <p role="status">{d.i18n.t(notice === 'saved' ? 'studio.settings.saved' : 'profile.edit.review')}</p>}
  </form>;
}
