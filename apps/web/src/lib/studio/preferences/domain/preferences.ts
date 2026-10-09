export interface ProfilePrivacy { userId: string; visibility: "public" | "private"; revision: number }
export interface EditableProfile { userId: string; username: string; displayName: string; description: string; revision: number }
export interface AccountIdentity { readonly id: string; readonly name: string; readonly email: string; readonly visibility: 'public' | 'private' }
export interface TimeZonePreference { readonly zone: string; readonly effectiveAt: number }
export interface TimeZoneChange { readonly reviewed: string; readonly proposed: string }
export interface Interactions { readonly comments: boolean; readonly reactions: boolean }
export interface NudgePreference { readonly enabled: boolean; readonly revision: number }
export interface BlockedPerson { readonly id: string; readonly name: string; readonly username: string }
export class PreferenceFailure extends Error {
  constructor(readonly kind: 'unavailable' | 'conflict' | 'rejected' = 'unavailable') { super('preference_failure'); }
}

export type NotificationChannel = 'following' | 'path_access' | 'tracking_activity' | 'achievements' | 'comments' | 'reactions' | 'comment_hearts' | 'nudges' | 'goal_reminders' | 'timer_health';
export interface NotificationChannelPreference { readonly channel: NotificationChannel; readonly enabled: boolean; readonly revision: number }

export interface TimerSubscriptionSubject { scope: 'person' | 'path'; id: string }
export interface TimerSubscriptionPreference { enabled: boolean; revision: number }

export interface ProfilePicture {userId:string;url:string;revision:number}
export interface PictureCrop {x:number;y:number;size:number}
export interface PicturePreview {image:string;width:number;height:number}
export interface PictureChange {userId:string;revision:number;image:string;crop?:PictureCrop;remove:boolean}

export interface WeekStartPreference { userId: string; firstDayOfWeek: number }
export interface WeekStartChange { userId: string; reviewedFirstDayOfWeek: number; proposedFirstDayOfWeek: number }

export interface GoalReminderSubject { id: string }
export interface GoalReminderPreference { enabled: boolean; revision: number }
