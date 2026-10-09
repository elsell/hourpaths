import { ReportAction } from './report-composer';
import { PathLeave } from './path-leave';
import { PathLifecycleReview } from './path-lifecycle';
import { reviewLifecycle, type LifecycleReview, type LifecycleAction } from '../paths/application/lifecycle';
import { PathAppearanceEditor } from './path-appearance';
import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import type { Path } from '../paths/domain/path';

export function PathActions({ path, dependencies: d, move, moving }: { path: Path; dependencies: StudioDependencies; move?: (direction: -1 | 1) => void; moving: boolean }) {
  const client = useQueryClient();
  const [lifecycle, setLifecycle] = useState<LifecycleReview | null>(null);
  const review = (action: LifecycleAction) => { setMenuOpen(false); setLifecycle(reviewLifecycle(path, action, d.operationId())); };
  const [menuOpen, setMenuOpen] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [appearance, setAppearance] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [name, setName] = useState(path.name);
  const refresh = async () => { await Promise.all([client.invalidateQueries({ queryKey: [d.accountScope, 'paths'] }), client.invalidateQueries({ queryKey: [d.accountScope, 'path', path.id] })]); };
  const rename = useMutation({ mutationFn: () => d.paths.rename(path, name.trim(), d.operationId()), onSuccess: async () => { setRenaming(false); await refresh(); } });
  const pin = useMutation({ mutationFn: () => d.paths.pin(path.id, !path.pinned, d.operationId()), onSuccess: refresh });
  return <div className="studio-path-actions">
    <details open={menuOpen}><summary onClick={event => { event.preventDefault(); setMenuOpen(!menuOpen); }} aria-label={d.i18n.t('home.pathActions', { pathName: path.name })}>⋯</summary><div className="studio-action-menu"><ReportAction target={{ kind: 'path', id: path.id }} dependencies={d} />
      {path.canManageVisibility && !path.archived && <Link to="/paths/$pathId/visibility" params={{ pathId: path.id }}>{d.i18n.t('pathVisibility.heading')}</Link>}
      {path.canTrack && <Link to="/paths/$pathId/nudge-settings" params={{ pathId: path.id }}>{d.i18n.t('nudge.audience.openLabel')}</Link>}<Link to="/paths/$pathId/people" params={{ pathId: path.id }}>{d.i18n.t('pathMembers.heading')}</Link>
      <Link to="/paths/$pathId/ownership" params={{ pathId: path.id }}>{d.i18n.t('pathOwnership.heading')}</Link>
      {path.canInvite && !path.archived && <Link to="/paths/$pathId/share" params={{ pathId: path.id }}>{d.i18n.t('pathInvitation.share')}</Link>}
      {path.canTrack && !path.archived && <Link to="/paths/$pathId/activities/new" params={{ pathId: path.id }}>{d.i18n.t('activity.add')}</Link>}
      {!path.archived && <button disabled={pin.isPending} onClick={() => { setMenuOpen(false); pin.mutate(); }}>{d.i18n.t(path.pinned ? 'home.arrange.unpinShort' : 'home.arrange.pinShort')}</button>}
      {path.canEdit && <button onClick={() => { setMenuOpen(false); setName(path.name); setRenaming(true); }}>{d.i18n.t('pathRename.action')}</button>}
      {move && <><button disabled={moving} onClick={() => { setMenuOpen(false); move(-1); }}>{d.i18n.t('home.arrange.moveUp', { pathName: path.name })}</button><button disabled={moving} onClick={() => { setMenuOpen(false); move(1); }}>{d.i18n.t('home.arrange.moveDown', { pathName: path.name })}</button></>}
      <button onClick={() => { setMenuOpen(false); setAppearance(true); }}>{d.i18n.t('home.appearance.title')}</button>
      {path.canLeave && !path.archived && <button onClick={() => { setMenuOpen(false); setLeaving(true); }}>{d.i18n.t('pathLeave.action')}</button>}
      {path.canManageLifecycle && <><button onClick={() => review(path.archived ? 'restore' : 'archive')}>{d.i18n.t(path.archived ? 'pathArchive.unarchiveAction' : 'pathArchive.action')}</button><button onClick={() => review('delete')}>{d.i18n.t('pathDelete.action')}</button></>}
    </div></details>
    {leaving && <PathLeave pathId={path.id} dependencies={d} close={() => setLeaving(false)} />}
    {lifecycle && <PathLifecycleReview review={lifecycle} dependencies={d} close={() => setLifecycle(null)} />}
    {appearance && <PathAppearanceEditor pathId={path.id} dependencies={d} close={() => setAppearance(false)} />}
    {renaming && <form className="studio-rename" onSubmit={event => { event.preventDefault(); rename.mutate(); }}>
      <label>{d.i18n.t('pathRename.nameLabel')}<input required maxLength={100} value={name} disabled={rename.isPending} onChange={event => setName(event.target.value)} /></label>
      <button type="submit" disabled={rename.isPending || !name.trim() || name.trim() === path.name}>{d.i18n.t('pathRename.save')}</button>
      <button type="button" disabled={rename.isPending} onClick={() => setRenaming(false)}>{d.i18n.t('common.cancel')}</button>
    </form>}
    {(pin.isError || rename.isError) && <p role="alert">{d.i18n.t('errors.apiRejected')}</p>}
  </div>;
}
