import type { AccountIdentity, TimeZonePreference, TimeZoneChange, Interactions, NudgePreference, BlockedPerson } from '../domain/preferences';
export interface PreferencesRepository {
  identity(signal?: AbortSignal): Promise<AccountIdentity>;
  timeZone(signal?: AbortSignal): Promise<TimeZonePreference>;
  changeTimeZone(change: TimeZoneChange, operationId: string, signal?: AbortSignal): Promise<TimeZonePreference>;
  interactions(signal?: AbortSignal): Promise<Interactions>;
  saveInteractions(value: Interactions, operationId: string, signal?: AbortSignal): Promise<Interactions>;
  nudges(signal?: AbortSignal): Promise<NudgePreference>;
  saveNudges(value: NudgePreference, operationId: string, signal?: AbortSignal): Promise<NudgePreference>;
  blocked(cursor?: string, signal?: AbortSignal): Promise<{ items: readonly BlockedPerson[]; next?: string }>;
  unblock(id: string, operationId: string, signal?: AbortSignal): Promise<void>;
}
