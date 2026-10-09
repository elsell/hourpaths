export interface EditableProfile {
  userId: string;
  username: string;
  displayName: string;
  description: string;
  revision: number;
}
export interface ProfileEditingRepository {
  read(): Promise<EditableProfile>;
  save(profile: EditableProfile, idempotencyKey: string): Promise<EditableProfile>;
}
export class ProfileEditFailure extends Error {
  constructor(readonly kind: 'invalid' | 'username_unavailable' | 'conflict' | 'rejected' | 'unavailable' = 'unavailable') { super(`profile_edit_${kind}`); }
}
export function normalizeEditableProfile(value: EditableProfile): EditableProfile {
  const profile = { ...value, displayName: value.displayName.trim(), description: value.description.trim() ? value.description : '' };
  if (!profile.userId || !/^[A-Za-z0-9_.]{3,64}$/.test(profile.username) ||
    [...profile.displayName].length < 1 || [...profile.displayName].length > 100 || [...profile.description].length > 500 ||
    !Number.isSafeInteger(profile.revision) || profile.revision < 1) throw new ProfileEditFailure('invalid');
  return profile;
}
export function createProfileEditOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let active: number | null = null;
  let retry: { signature: string; key: string } | undefined;
  return {
    async submit(value: EditableProfile, request: (value: EditableProfile, key: string) => Promise<EditableProfile>): Promise<
      { kind: 'applied'; profile: EditableProfile } | { kind: 'failed'; cause: unknown } | { kind: 'superseded' | 'busy' }
    > {
      if (active !== null) return { kind: 'busy' };
      const generation = ++epoch;
      active = generation;
      try {
        const frozen = normalizeEditableProfile(value);
        const signature = JSON.stringify([frozen.userId, frozen.revision, frozen.username, frozen.displayName, frozen.description]);
        const attempt = retry?.signature === signature ? retry : { signature, key: keyFactory() };
        retry = attempt;
        const profile = await request(frozen, attempt.key);
        if (epoch !== generation) return { kind: 'superseded' };
        if (profile.userId !== frozen.userId || profile.username !== frozen.username || profile.displayName !== frozen.displayName || profile.description !== frozen.description || profile.revision !== frozen.revision + 1) throw new ProfileEditFailure();
        retry = undefined;
        return { kind: 'applied', profile };
      } catch (cause) {
        return epoch === generation ? { kind: 'failed', cause } : { kind: 'superseded' };
      } finally { if (active === generation) active = null; }
    },
    cancel() { epoch += 1; active = null; retry = undefined; },
  };
}
