export interface BlockIdentity { userId: string; username: string; displayName: string }
export interface BlockReview {
  target: BlockIdentity;
  sharedPaths: readonly { id: string; name: string }[];
  acknowledgement: { version: 1; token: string; expiresAt: string };
}
