import { ianaGeographicZones } from './iana-geographic-zones';

function runtimeSupportsTimeZone(zone: string): boolean {
  if (!zone || zone.trim() !== zone || zone === 'Local') return false;
  try { new Intl.DateTimeFormat('en', { timeZone: zone }).format(0); return true; }
  catch { return false; }
}

/** Enumeration is absent in supported native runtimes. Use the same reviewed
 * catalog across clients, with runtime validation and saved-alias preservation. */
export function timeZoneChoices(
  configured?: string,
  supported: (zone: string) => boolean = runtimeSupportsTimeZone,
): string[] {
  const choices = new Set(ianaGeographicZones);
  if (configured) choices.add(configured);
  return [...choices].filter(supported).sort();
}
