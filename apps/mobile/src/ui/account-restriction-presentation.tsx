import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import type { AccountRestrictionController, AccountExportController, EnforcementRepository } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { NativeSheet, ActionButton, ThemedText } from './primitives';
import { SettingsShell, SettingsSection } from './settings-list';
import { EnforcementSettingsView } from './enforcement-settings-view';
import { AccountExport } from './account-export';

type Presentation = { id: number; controller: AccountRestrictionController; i18n: Translator; repository: EnforcementRepository; exportData?: AccountExportController; signOut(): Promise<void>; deleteAccount(): Promise<void> };
const Context = createContext<((source: Presentation) => () => void) | null>(null);
let sequence = 0;
export function NativeAccountRestrictionProvider({ children }: { children: ReactNode }) {
  const [source, setSource] = useState<Presentation | null>(null);
  const register = useRef((next: Presentation) => { setSource(next); return () => setSource(current => current?.id === next.id ? null : current); }).current;
  return <Context.Provider value={register}>{children}{source && <RestrictionHost key={source.id} source={source} />}</Context.Provider>;
}
export function NativeAccountRestrictionSource(props: Omit<Presentation, 'id'>) {
  const register = useContext(Context), current = useRef(props); current.current = props;
  useEffect(() => register?.({ id: ++sequence, controller: props.controller, i18n: props.i18n, exportData: props.exportData,
    repository: { list: cursor => current.current.repository.list(cursor), get: id => current.current.repository.get(id), appeal: (id, text, key) => current.current.repository.appeal(id, text, key) },
    signOut: () => current.current.signOut(), deleteAccount: () => current.current.deleteAccount(),
  }), [register, props.controller, props.i18n, props.exportData]);
  return null;
}
function RestrictionHost({ source }: { source: Presentation }) {
  const [state, setState] = useState(source.controller.state), [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  const [exportBusy, setExportBusy] = useState(false);
  useEffect(() => { setState(source.controller.state); return source.controller.subscribe(() => setState(source.controller.state)); }, [source.controller]);
  useEffect(() => source.exportData?.subscribe(() => setExportBusy(['collecting', 'saving'].includes(source.exportData!.state.phase))), [source.exportData]);
  useEffect(() => {
    const until = state.review?.restriction?.until;
    if (!until || !state.required) return;
    const delay = Date.parse(until) - Date.now();
    if (delay <= 0) return;
    const timer = setTimeout(() => void source.controller.refresh(), Math.min(delay + 100, 2_147_483_647));
    return () => clearTimeout(timer);
  }, [source.controller, state.review]);
  async function run(action: () => Promise<void>) {
    if (busy || exportBusy) return;
    setBusy(true); setFailed(false);
    try { await action(); } catch { setFailed(true); } finally { setBusy(false); }
  }
  if (!state.required) return null;
  return <NativeSheet visible={state.required} dismissible={false} scrollable={false} title={source.i18n.t('accountRestriction.heading')} onRequestClose={() => {}}>
    <SettingsShell>
      <ThemedText>{source.i18n.t('accountRestriction.body')}</ThemedText>
      {(failed || state.error) && <ThemedText accessibilityRole="alert">{source.i18n.t('errors.temporarilyUnavailable')}</ThemedText>}
      <ActionButton variant="quiet" label={source.i18n.t('common.refresh')} disabled={state.loading} onPress={() => void source.controller.refresh()} />
      <EnforcementSettingsView i18n={source.i18n} repository={source.repository} embedded />
      {source.exportData && <AccountExport controller={source.exportData} i18n={source.i18n} />}
      <SettingsSection>
        <ActionButton variant="secondary" label={source.i18n.t('auth.signOut')} disabled={busy || exportBusy} onPress={() => void run(source.signOut)} />
        <ActionButton variant="quiet" label={source.i18n.t('accountDelete.heading')} disabled={busy || exportBusy} onPress={() => void run(source.deleteAccount)} />
      </SettingsSection>
    </SettingsShell>
  </NativeSheet>;
}
