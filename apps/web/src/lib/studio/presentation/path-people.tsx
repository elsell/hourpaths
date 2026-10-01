import { useState } from 'react';
import { Link, useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { StudioShell } from './studio-shell';
import { MemberAccess } from './member-access';
export function PathPeoplePage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const { pathId } = useParams({ strict: false }) as { pathId: string };
  return <PathPeople key={pathId} pathId={pathId} dependencies={d} />;
}
function PathPeople({ pathId, dependencies: d }: { pathId: string; dependencies: StudioDependencies }) {
  const [busy, setBusy] = useState(false);
  const query = useQuery({ queryKey: [d.accountScope, 'pathPeople', pathId], queryFn: ({ signal }) => d.paths.read(pathId, signal), staleTime: 0, gcTime: 0, refetchOnWindowFocus: true });
  const path = !query.isError ? query.data : undefined;
  return <StudioShell page="paths" i18n={d.i18n}><main className="studio-main studio-activity-detail"><header className="studio-header"><div><Link to="/paths/$pathId" params={{ pathId }}>{d.i18n.t('studio.path')}</Link><h1>{d.i18n.t('pathMembers.heading')}</h1>{path && <p>{path.name}</p>}</div></header>
    {query.isPending ? <p role="status">{d.i18n.t('common.loading')}</p> : !path ? <div role="alert"><p>{d.i18n.t('studio.loadFailed')}</p><button disabled={busy} onClick={() => void query.refetch()}>{d.i18n.t('common.retry')}</button></div> : <MemberAccess dependencies={d} pathId={pathId} mode="people" archived={path.archived} disabled={false} onBusy={setBusy} />}
  </main></StudioShell>;
}
