import { useEffect, useState } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { RetainedTimers, RetainedTimersSnapshot } from '../offline/ports/retained-timers';
import { duration } from './duration';

export function RetainedTimerControls({ service, i18n }: { service: RetainedTimers; i18n: Translator }) {
  const [state, setState] = useState<RetainedTimersSnapshot | null>(null);
  const [now, setNow] = useState(Date.now);
  const [busy, setBusy] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let active = true;
    void service.snapshot().then(value => { if (active && service.valid()) setState(value); }).catch(() => { if (active) setFailed(true); });
    const timer = setInterval(() => { setNow(Date.now()); if (!service.valid()) setState(null); }, 1000);
    return () => { active = false; clearInterval(timer); };
  }, [service]);
  if (!service.valid()) return null;
  async function stop(id: string) {
    if (busy) return;
    setBusy(id); setFailed(false);
    try { const value = await service.stop(id); if (service.valid()) setState(value); }
    catch { if (service.valid()) setFailed(true); }
    finally { setBusy(null); }
  }
  return <section className="studio-retained-timers" aria-label={i18n.t('offline.signInRequired')}>
    <p>{i18n.t('offline.signInRequiredBody')}</p>
    {state?.pending && <p role="status">{i18n.t('offline.pending')}</p>}
    {state?.timers.map(timer => <article className="studio-retained-timer" key={timer.id}>
      <div><strong>{timer.name || i18n.t('offline.retainedPath')}</strong><p>{duration(i18n, Math.max(0, Math.floor((now - timer.startedAt) / 1000)))}</p></div>
      <button disabled={busy !== null} onClick={() => void stop(timer.id)}>{i18n.t('timer.stop')}</button>
    </article>)}
    {failed && <p role="alert">{i18n.t('errors.temporarilyUnavailable')}</p>}
  </section>;
}
