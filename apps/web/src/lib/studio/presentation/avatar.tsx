import type { Person } from '../social/domain/activity';

export function Avatar({ person, segments = 0 }: { person: Person; segments?: number }) {
  const circumference = 2 * Math.PI * 46;
  const gap = segments > 1 ? Math.min(6, circumference / segments / 4) : 0;
  return <span className="studio-avatar">
    {segments > 0 && <svg viewBox="0 0 100 100" aria-hidden="true" className="studio-avatar-ring"><circle cx="50" cy="50" r="46" fill="none" stroke="currentColor" strokeWidth="5" strokeDasharray={[circumference / segments - gap, gap].join(" ")} transform="rotate(-90 50 50)" /></svg>}
    {person.picture ? <img src={person.picture} alt="" crossOrigin="anonymous" referrerPolicy="no-referrer" /> : <svg className="studio-avatar-placeholder" viewBox="0 0 64 64" aria-hidden="true"><circle cx="32" cy="23" r="11" /><path d="M10 57c0-15 9-22 22-22s22 7 22 22" /></svg>}
  </span>;
}
