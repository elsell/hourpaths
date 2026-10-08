import { createTimerSubscriptionOperationOwner, TimerSubscriptionFailure, type TimerSubscriptionPreference, type TimerSubscriptionSubject } from '@hourpaths/client-core';
import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { StudioDependencies } from './app';
import { useOwnedOperation } from './use-owned-operation';

export function TimerSubscription({ subject, dependencies: d }: { subject: TimerSubscriptionSubject; dependencies: StudioDependencies }) {
  const client = useQueryClient();
  const operation = useOwnedOperation(d);
  const [owner] = useState(() => createTimerSubscriptionOperationOwner(d.operationId));
  useEffect(() => () => owner.cancel(), [owner]);
  const key = [d.accountScope, 'timer-subscription', subject.scope, subject.id];
  const query = useQuery({ queryKey: key, queryFn: ({ signal }) => d.preferences.timerSubscription(subject, signal), refetchOnWindowFocus: true });
  const mutation = useMutation({
    mutationFn: (value: TimerSubscriptionPreference) => operation.run(async signal => {
      const result = await owner.submit(subject, value, (target, next, id) => d.preferences.saveTimerSubscription(target, next, id, signal));
      if (result.kind === 'applied') return result.preference;
      if (result.kind === 'failed') throw result.cause;
      throw new TimerSubscriptionFailure();
    }),
    onSuccess: value => { if (operation.active()) client.setQueryData(key, value); },
  });
  const conflict = mutation.error instanceof TimerSubscriptionFailure && mutation.error.kind === 'conflict';
  const label = d.i18n.t(subject.scope === 'person' ? 'timerSubscription.person' : 'timerSubscription.path');
  return <section className="studio-settings-section" aria-label={label}>
    <label className="studio-setting-row"><span>{label}</span><input type="checkbox" checked={query.data?.enabled ?? false} disabled={!query.data || query.isError || query.isFetching || mutation.isPending || conflict} onChange={event => { if (query.data) mutation.mutate({ ...query.data, enabled: event.target.checked }); }} /></label>
    <p>{d.i18n.t('timerSubscription.channelHint')}</p>
    {(query.isError || mutation.isError) && <p role="alert">{d.i18n.t(conflict ? 'studio.settings.conflict' : 'timerSubscription.failed')} <button disabled={mutation.isPending} onClick={() => {
      if (!conflict && mutation.variables) mutation.mutate(mutation.variables);
      else { owner.cancel(); mutation.reset(); void query.refetch(); }
    }}>{d.i18n.t(conflict ? 'common.refresh' : 'common.retry')}</button></p>}
  </section>;
}
