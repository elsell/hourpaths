import { useEffect, useState } from 'react';
import type { AccountExportController } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';

export function AccountExport({ controller, i18n }: { controller: AccountExportController; i18n: Translator }) {
  const [state, setState] = useState(controller.state);
  useEffect(() => { setState(controller.state); return controller.subscribe(() => setState(controller.state)); }, [controller]);
  const busy = state.phase === 'collecting' || state.phase === 'saving';
  return <section className="studio-retained-timers" aria-label={i18n.t('accountExport.heading')}>
    <button type="button" disabled={busy} onClick={() => void controller.save()}>{i18n.t('accountExport.heading')}</button>
    {busy && <p role="status">{i18n.t(state.phase === 'collecting' ? 'accountExport.preparing' : 'accountExport.saving')}</p>}
    {state.phase === 'collecting' && <button type="button" onClick={() => controller.cancel()}>{i18n.t('common.cancel')}</button>}
    {state.phase === 'failed' && <p role="alert">{i18n.t('accountExport.failed')}</p>}
    {state.phase === 'saved' && <p role="status">{i18n.t('accountExport.ready')}</p>}
  </section>;
}
