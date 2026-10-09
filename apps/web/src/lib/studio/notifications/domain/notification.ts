export type NotificationTarget = { readonly kind: 'reminder'; readonly paths: readonly Readonly<{ id: string; name: string }>[] } | { readonly kind: 'invitations' | 'people' } | { readonly kind: 'ownership' | 'path'; readonly pathId: string } | { readonly kind: 'profile'; readonly username: string };
export interface Notification {
  readonly id: string;
  readonly reportTarget?: { readonly kind: 'nudge'; readonly id: string };
  readonly message: string;
  readonly createdAt: number;
  readonly read: boolean;
  readonly presentation: 'actionable' | 'informational';
  readonly target: NotificationTarget | null;
}
export interface NotificationsSnapshot {
  readonly items: readonly Notification[];
  readonly unreadCount: number;
  readonly nextCursor: string;
  readonly loaded: boolean;
  readonly loading: boolean;
  readonly mutating: boolean;
  readonly error: 'history' | 'mutation' | null;
}
export type NotificationChange = { readonly kind: 'read' | 'delete'; readonly notificationId: string } | { readonly kind: 'read-all' };
