import { useEffect, useState } from 'react';
import type { PolicyTimersController } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { ActionButton, ThemedText } from './primitives';
import { SettingsSection } from './settings-list';

export function PolicyTimers({ controller, i18n }: { controller: PolicyTimersController; i18n: Translator }) {
  const [state, setState] = useState(controller.state);
  useEffect(() => {
    setState(controller.state);
    const unsubscribe = controller.subscribe(() => setState(controller.state));
    void controller.refresh();
    return unsubscribe;
  }, [controller]);
  return <SettingsSection title={i18n.t('policyReview.timers')}>
    {state.timers.map(timer => <ActionButton variant="secondary" key={timer.source + timer.id}
      busy={state.stopping === timer.id} disabled={state.loading || !!state.stopping}
      label={i18n.t('policyReview.stopTimer', { name: timer.name || i18n.t('offline.retainedPath') })}
      onPress={() => void controller.stop(timer.id, timer.source)} />)}
    {state.loading && <ThemedText accessibilityLiveRegion="polite">{i18n.t('common.loading')}</ThemedText>}
    {!state.loading && !state.error && !state.timers.length && !state.pending && <ThemedText>{i18n.t('policyReview.noTimers')}</ThemedText>}
    {state.pending && <ThemedText accessibilityLiveRegion="polite">{i18n.t('policyReview.pendingStops')}</ThemedText>}
    {state.error && <ThemedText accessibilityRole="alert">{i18n.t('errors.temporarilyUnavailable')}</ThemedText>}
    <ActionButton variant="quiet" label={i18n.t(state.error ? 'common.retry' : 'common.refresh')} disabled={state.loading || !!state.stopping} onPress={() => void controller.refresh()} />
    {state.nextCursor && <ActionButton variant="secondary" label={i18n.t('policyReview.moreTimers')} disabled={state.loading || !!state.stopping} onPress={() => void controller.refresh(true)} />}
  </SettingsSection>;
}
