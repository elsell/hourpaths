import { createSessionApiClient } from '@hourpaths/api-client';
import { ProfilePrivacyFailure, validProfilePrivacy, type ProfilePrivacy, type ProfilePrivacyRepository } from '../profile-privacy';
function profileFromAPI(value: unknown, owner: string): ProfilePrivacy {
  if (!value || typeof value !== 'object') throw new ProfilePrivacyFailure();
  const row = value as Partial<ProfilePrivacy>;
  if (row.userId !== owner || (row.visibility !== 'public' && row.visibility !== 'private') || typeof row.revision !== 'number') throw new ProfilePrivacyFailure();
  const profile = { userId: owner, visibility: row.visibility, revision: row.revision };
  if (!validProfilePrivacy(profile)) throw new ProfilePrivacyFailure();
  return profile;
}
function failure(status: number): ProfilePrivacyFailure {
  return new ProfilePrivacyFailure(status === 409 ? 'conflict' : status === 400 || status === 422 ? 'invalid' : status === 401 || status === 403 ? 'rejected' : 'unavailable');
}
export function apiProfilePrivacy(baseURL: string, token: string | null, owner: string, rejected?: (token: string | null) => void, signal?: AbortSignal): ProfilePrivacyRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async read() {
      const result = await api.ownProfilePrivacy();
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return profileFromAPI(result.data.data, owner);
    },
    async save(reviewed, visibility, key) {
      if (!validProfilePrivacy(reviewed) || reviewed.userId !== owner) throw new ProfilePrivacyFailure('rejected');
      const result = await api.updateOwnProfilePrivacy({ visibility, expectedRevision: reviewed.revision, confirmed: true }, key);
      if (!result.response.ok || !result.data) throw failure(result.response.status);
      return profileFromAPI(result.data.data, owner);
    },
  };
}
