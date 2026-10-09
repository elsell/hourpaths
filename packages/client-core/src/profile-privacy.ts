export type ProfileVisibility = 'public' | 'private';
export interface ProfilePrivacy { userId: string; visibility: ProfileVisibility; revision: number }
export interface ProfilePrivacyRepository {
  read(): Promise<ProfilePrivacy>;
  save(reviewed: ProfilePrivacy, visibility: ProfileVisibility, key: string): Promise<ProfilePrivacy>;
}
export class ProfilePrivacyFailure extends Error {
  constructor(readonly kind: 'invalid' | 'conflict' | 'rejected' | 'unavailable' = 'unavailable') { super(`profile_privacy_${kind}`); }
}
export function validProfilePrivacy(value: ProfilePrivacy): boolean {
  return !!value.userId && (value.visibility === 'public' || value.visibility === 'private') && Number.isSafeInteger(value.revision) && value.revision > 0;
}
/** Own a reviewed transition, retaining its retry identity until completion or cancellation. */
export function createProfilePrivacyOperationOwner(keyFactory: () => string) {
  let epoch = 0, active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(reviewed: ProfilePrivacy, visibility: ProfileVisibility, request: ProfilePrivacyRepository['save']): Promise<
      { kind: 'applied'; profile: ProfilePrivacy } | { kind: 'failed'; cause: unknown } | { kind: 'superseded' | 'busy' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch; active = generation;
      try {
        const frozen = { ...reviewed };
        if (!validProfilePrivacy(frozen) || !['public', 'private'].includes(visibility) || frozen.visibility === visibility) throw new ProfilePrivacyFailure('invalid');
        const signature = JSON.stringify([frozen.userId, frozen.revision, frozen.visibility, visibility]);
        const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() };
        retry = attempt;
        const profile = await request(frozen, visibility, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (!validProfilePrivacy(profile) || profile.userId !== frozen.userId || profile.visibility !== visibility || profile.revision !== frozen.revision + 1) throw new ProfilePrivacyFailure();
        retry = undefined;
        return { kind: 'applied', profile };
      } catch (cause) { return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' }; }
      finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
