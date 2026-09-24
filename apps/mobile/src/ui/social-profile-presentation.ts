export type SocialPublicProfile = {
  description?: string;
  displayName: string;
  followerCount: number;
  followingCount: number;
  profilePictureUrl?: string;
  relationship: 'self' | 'none' | 'requested' | 'following';
  userId: string;
  username: string;
};

export function eligibleProfileSearchQuery(query: string): string | undefined {
  const trimmed = query.trim();
  return [...trimmed].length >= 2 ? trimmed : undefined;
}

export function profileAccessibilityLabel(profile: Pick<SocialPublicProfile, 'displayName' | 'username'>) {
  return `${profile.displayName}, @${profile.username}`;
}
