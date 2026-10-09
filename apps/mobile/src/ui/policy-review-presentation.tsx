import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { PolicyReviewController } from '@hourpaths/client-core';
import { ActionButton, NativeSheet, ThemedText } from './primitives';
import { SettingsSection } from './settings-list';
import { PolicyReviewView } from './policy-review-view';

type Presentation = { id: number; controller: PolicyReviewController; i18n: Translator; signOut(): Promise<void>; deleteAccount(): Promise<void> };
let presentationSequence = 0;
const PolicyContext = createContext<((source: Presentation) => () => void) | null>(null);
export function NativePolicyReviewProvider({ children }: { children: ReactNode }) {
  const [source, setSource] = useState<Presentation | null>(null);
  const register = useRef((next: Presentation) => {
    setSource(next);
    return () => setSource(current => current?.id === next.id ? null : current);
  }).current;
  return <PolicyContext.Provider value={register}>{children}{source && <PolicyReviewHost key={source.id} source={source} />}</PolicyContext.Provider>;
}
export function NativePolicyReviewSource({ controller, i18n, signOut, deleteAccount }: Omit<Presentation, 'id'>) {
  const register = useContext(PolicyContext);
  const actions = useRef({ signOut, deleteAccount }); actions.current = { signOut, deleteAccount };
  useEffect(() => register?.({ id: ++presentationSequence, controller, i18n, signOut: () => actions.current.signOut(), deleteAccount: () => actions.current.deleteAccount() }), [controller, i18n, register]);
  return null;
}
function PolicyReviewHost({ source }: { source: Presentation }) {
  const [state, setState] = useState(source.controller.state);
  const [actionBusy, setActionBusy] = useState(false);
  const [actionFailed, setActionFailed] = useState(false);
  useEffect(() => { setState(source.controller.state); return source.controller.subscribe(() => setState(source.controller.state)); }, [source.controller]);
  async function run(action: () => Promise<void>) {
    if (actionBusy) return;
    setActionBusy(true); setActionFailed(false);
    try { await action(); } catch { setActionFailed(true); }
    finally { setActionBusy(false); }
  }
  return <NativeSheet visible={state.required} dismissible={false} scrollable={false} title={source.i18n.t('policyReview.heading')} onRequestClose={() => {}}>
    <PolicyReviewView controller={source.controller} i18n={source.i18n} showHeading={false} actions={<SettingsSection title={source.i18n.t('policyReview.accountOptions')}>
      {actionFailed && <ThemedText accessibilityRole="alert">{source.i18n.t('errors.temporarilyUnavailable')}</ThemedText>}
      <ActionButton variant="secondary" label={source.i18n.t('settings.account.activeTimers.keepRunningAndSignOut')} disabled={actionBusy || state.saving || ['collecting', 'saving'].includes(source.controller.exportData?.state.phase ?? '')} onPress={() => void run(source.signOut)} />
      <ActionButton variant="quiet" label={source.i18n.t('accountDelete.heading')} disabled={actionBusy || state.saving || ['collecting', 'saving'].includes(source.controller.exportData?.state.phase ?? '')} onPress={() => void run(source.deleteAccount)} />
    </SettingsSection>} />
  </NativeSheet>;
}
