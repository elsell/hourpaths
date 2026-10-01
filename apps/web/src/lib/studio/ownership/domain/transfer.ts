export interface Identity { readonly userId: string; readonly username: string; readonly displayName: string }
export interface Candidate extends Identity { readonly administrator: boolean }
export interface TransferReview {
  readonly recipient: Identity;
  readonly reservationToken: string;
  readonly reviewedAt: string;
  readonly expiresAt: string;
  readonly viewerTimeZone: string;
}
export interface Transfer {
  readonly id: string;
  readonly pathId: string;
  readonly creatorUserId: string;
  readonly recipientUserId: string;
  readonly reviewedAt: string;
  readonly expiresAt: string;
  readonly state: 'pending' | 'accepted' | 'declined' | 'canceled';
  readonly counterpart: Identity;
  readonly counterpartRole: 'creator' | 'recipient';
  readonly viewerTimeZone: string;
}
export type TransferCommand = { readonly kind: 'initiate'; readonly pathId: string; readonly review: TransferReview }
  | { readonly kind: 'accept' | 'decline' | 'cancel'; readonly transfer: Transfer };
