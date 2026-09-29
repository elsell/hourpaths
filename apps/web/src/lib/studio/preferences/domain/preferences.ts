export interface AccountIdentity { readonly id: string; readonly name: string; readonly email: string; readonly visibility: 'public' | 'private' }
export interface TimeZonePreference { readonly zone: string; readonly effectiveAt: number }
export interface TimeZoneChange { readonly reviewed: string; readonly proposed: string }
export interface Interactions { readonly comments: boolean; readonly reactions: boolean }
export interface NudgePreference { readonly enabled: boolean; readonly revision: number }
export interface BlockedPerson { readonly id: string; readonly name: string; readonly username: string }
export class PreferenceFailure extends Error {
  constructor(readonly kind: 'unavailable' | 'conflict' | 'rejected' = 'unavailable') { super('preference_failure'); }
}
