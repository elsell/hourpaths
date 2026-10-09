import type { TimerSubscriptionSubject, TimerSubscriptionPreference } from '../domain/preferences';
import type { NotificationChannelPreference, AccountIdentity, TimeZonePreference, TimeZoneChange, Interactions, NudgePreference, BlockedPerson } from '../domain/preferences';
export interface PreferencesRepository {
  weekStart(owner: string, signal?: AbortSignal): Promise<import('../domain/preferences').WeekStartPreference>;
  saveWeekStart(value: import('../domain/preferences').WeekStartChange, key: string, signal?: AbortSignal): Promise<import('../domain/preferences').WeekStartPreference>;
  profilePicture(owner:string,signal?:AbortSignal):Promise<import('../domain/preferences').ProfilePicture>;
  previewPicture(owner:string,image:string,signal?:AbortSignal):Promise<import('../domain/preferences').PicturePreview>;
  savePicture(value:import('../domain/preferences').PictureChange,key:string,signal?:AbortSignal):Promise<import('../domain/preferences').ProfilePicture>;
  profilePrivacy(owner: string, signal?: AbortSignal): Promise<import('../domain/preferences').ProfilePrivacy>;
  saveProfilePrivacy(value: import('../domain/preferences').ProfilePrivacy, visibility: "public" | "private", key: string, signal?: AbortSignal): Promise<import('../domain/preferences').ProfilePrivacy>;
  editableProfile(owner: string, signal?: AbortSignal): Promise<import('../domain/preferences').EditableProfile>;
  saveProfile(value: import('../domain/preferences').EditableProfile, key: string, signal?: AbortSignal): Promise<import('../domain/preferences').EditableProfile>;
  timerSubscription(subject: TimerSubscriptionSubject, signal?: AbortSignal): Promise<TimerSubscriptionPreference>;
  saveTimerSubscription(subject: TimerSubscriptionSubject, value: TimerSubscriptionPreference, operationId: string, signal?: AbortSignal): Promise<TimerSubscriptionPreference>;
  notificationChannels(signal?: AbortSignal): Promise<NotificationChannelPreference[]>;
  saveNotificationChannel(value: NotificationChannelPreference, operationId: string, signal?: AbortSignal): Promise<NotificationChannelPreference>;
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
