import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { emojiChoices } from '@hourpaths/i18n';
import type { StudioDependencies } from './app';
import type { PathAppearance } from '../paths/domain/path';

export function PathAppearanceEditor({ pathId, dependencies: d, close }: { pathId: string; dependencies: StudioDependencies; close(): void }) {
  const query = useQuery({ queryKey: [d.accountScope, 'appearance', pathId], queryFn: ({ signal }) => d.paths.appearance(pathId, signal) });
  if (!query.data) return <div className="studio-appearance-editor"><p>{d.i18n.t(query.isError ? 'home.appearance.loadFailed' : 'studio.loading')}</p><button onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button><button onClick={close}>{d.i18n.t('common.cancel')}</button></div>;
  return <AppearanceForm initial={query.data} pathId={pathId} dependencies={d} close={close} />;
}
function AppearanceForm({ initial, pathId, dependencies: d, close }: { initial: PathAppearance; pathId: string; dependencies: StudioDependencies; close(): void }) {
  const client = useQueryClient();
  const [draft, setDraft] = useState(initial);
  const [search, setSearch] = useState('');
  const [limit, setLimit] = useState(60);
  const choices = emojiChoices.filter(item => d.i18n.t(item.key).toLocaleLowerCase(d.i18n.locale).includes(search.toLocaleLowerCase(d.i18n.locale)) || item.emoji.includes(search));
  const mutation = useMutation({
    mutationFn: () => d.paths.saveAppearance(pathId, draft, d.operationId()),
    onSuccess: async () => { await client.invalidateQueries({ queryKey: [d.accountScope, 'appearance', pathId] }); close(); },
    onError: async () => {
      // Preserve the draft. Applying it over another client's edit requires the
      // user's explicit retry after the existing conflict/retry message.
      try {
        const latest = await d.paths.appearance(pathId);
        setDraft(current => ({ ...current, revision: latest.revision }));
      } catch { /* A later retry must still satisfy the server revision check. */ }
    },
  });
  return <form className="studio-appearance-editor studio-form" onSubmit={event => { event.preventDefault(); mutation.mutate(); }}>
    <h2>{d.i18n.t('home.appearance.title')}</h2>
    <fieldset disabled={mutation.isPending}>
      <label>{d.i18n.t('home.appearance.color')}<select value={draft.color} onChange={event => setDraft({ ...draft, color: event.target.value as PathAppearance['color'] })}>{(['coral', 'lavender', 'gold', 'mint', 'blue', 'pink'] as const).map(color => <option key={color} value={color}>{d.i18n.t(`home.appearance.color.${color}`)}</option>)}</select></label>
      <label>{d.i18n.t('emojiPicker.search')}<input type="search" value={search} onChange={event => { setSearch(event.target.value); setLimit(60); }} /></label>
      <div className="studio-emoji-choices" aria-label={d.i18n.t('home.appearance.emoji')}>{choices.slice(0, limit).map(choice => <button type="button" key={choice.key} title={d.i18n.t(choice.key)} aria-label={d.i18n.t(choice.key)} aria-pressed={draft.emoji === choice.emoji} onClick={() => setDraft({ ...draft, emoji: choice.emoji })}>{choice.emoji}</button>)}</div>
      {!choices.length && <p>{d.i18n.t('emojiPicker.empty')}</p>}
      {choices.length > limit && <button type="button" onClick={() => setLimit(limit + 60)}>{d.i18n.t('studio.moreEmoji')}</button>}
      <div className="studio-form-actions"><button type="button" onClick={close}>{d.i18n.t('common.cancel')}</button><button className="studio-primary" type="submit">{draft.emoji} {d.i18n.t('common.save')}</button></div>
    </fieldset>
    {mutation.isError && <p role="alert">{d.i18n.t('home.appearance.saveFailed')}</p>}
  </form>;
}
