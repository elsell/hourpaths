import { ConnectedTimeline } from './connected-timeline';
import { duration } from './duration';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import type { Translator } from '@hourpaths/i18n';
import type { PathRepository } from '../paths/ports/path-repository';
import type { HistoryRepository } from '../history/ports/history-source';
import type { HistoryCursor, RecordedActivity } from '../history/domain/activity';

export function ActivityTimeline({ paths, history, accountScope, i18n }: { paths: PathRepository; history: HistoryRepository; accountScope: string; i18n: Translator }) {
  const query = useInfiniteQuery({
    queryKey: [accountScope, 'history'], initialPageParam: null as HistoryCursor | null,
    queryFn: ({ pageParam, signal }) => history.page(pageParam, signal),
    getNextPageParam: page => page.next ?? undefined,
  });
  const entries = [...new Map((query.data?.pages.flatMap(page => page.items) ?? []).map(item => [item.id, item])).values()];
  return <section className="studio-history" aria-label={i18n.t('studio.activity')}>
    <header><h2>{i18n.t('studio.activity')}</h2><button onClick={() => void query.refetch()} disabled={query.isFetching}>{i18n.t('studio.refresh')}</button></header>
    {query.isPending && <p role="status">{i18n.t('studio.loading')}</p>}
    {!query.isPending && !query.isError && !entries.length && <p>{i18n.t('studio.noActivity')}</p>}
    <ConnectedTimeline items={entries} identity={entry => entry.id} instant={entry => entry.startedAt} timeZone={entry => entry.timeZone} i18n={i18n} render={entry => <>
      <time dateTime={new Date(entry.startedAt).toISOString()}>{i18n.time(entry.startedAt, { hour: 'numeric', minute: '2-digit', timeZone: entry.timeZone })}</time>
      <TimelinePath entry={entry} paths={paths} accountScope={accountScope} i18n={i18n} />
    </>} />
    {query.isError && <div role="alert"><p>{i18n.t('studio.loadFailed')}</p><button onClick={() => void query.refetch()}>{i18n.t('common.retry')}</button></div>}
    {query.hasNextPage && <button disabled={query.isFetching} onClick={() => void query.fetchNextPage()}>{i18n.t('studio.moreHistory')}</button>}
  </section>;
}

function TimelinePath({ entry, paths, accountScope, i18n }: { entry: RecordedActivity; paths: PathRepository; accountScope: string; i18n: Translator }) {
  const appearance = useQuery({ queryKey: [accountScope, 'appearance', entry.pathId], queryFn: ({ signal }) => paths.appearance(entry.pathId, signal) });
  return <div className="studio-timeline-identity"><span className="studio-emoji" data-color={appearance.data?.color} aria-hidden="true">{appearance.data?.emoji ?? '✨'}</span><div><strong>{entry.pathName}</strong><small>{duration(i18n, entry.seconds)}</small></div></div>;
}
