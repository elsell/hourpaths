import type { ReactNode } from 'react';
import type { Translator } from '@hourpaths/i18n';

export function ConnectedTimeline<T>({ items, identity, instant, timeZone, render, i18n, className = '' }: {
  items: readonly T[]; identity(item: T): string; instant(item: T): number;
  timeZone(item: T): string; render(item: T): ReactNode; i18n: Translator; className?: string;
}) {
  let previous = '';
  return <ol className={["studio-timeline", className].join(" ")}>{items.flatMap(item => {
    const at = instant(item);
    const zone = timeZone(item);
    const parts = new Intl.DateTimeFormat('en', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: zone }).formatToParts(at);
    const day = ['year', 'month', 'day'].map(type => parts.find(part => part.type === type)?.value).join('-');
    const heading = day !== previous;
    previous = day;
    return [heading ? <li className="studio-timeline-day" key={["day", identity(item)].join("-")}><span className="studio-day-dot" /><h3>{i18n.date(at, { month: 'long', day: 'numeric', year: 'numeric', timeZone: zone })}</h3></li> : null,
      <li key={identity(item)} className="studio-timeline-event"><span className="studio-event-dot" />{render(item)}</li>];
  })}</ol>;
}
