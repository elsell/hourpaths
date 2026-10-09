import { createSessionApiClient } from '@hourpaths/api-client';
import { ProfileEditFailure, normalizeEditableProfile, type EditableProfile, type ProfileEditingRepository } from '../profile-editing';

export function editableProfileFromAPI(value: unknown, owner: string): EditableProfile {
  if (!value || typeof value !== 'object') throw new ProfileEditFailure();
  const row = value as Partial<EditableProfile>;
  if (row.userId !== owner || typeof row.username !== 'string' || typeof row.displayName !== 'string' || typeof row.description !== 'string' || typeof row.revision !== 'number') throw new ProfileEditFailure();
  return normalizeEditableProfile({ userId: owner, username: row.username, displayName: row.displayName, description: row.description, revision: row.revision });
}
function failure(status: number, code?: string): ProfileEditFailure {
  return new ProfileEditFailure(code === 'username_unavailable' ? 'username_unavailable' : status === 409 ? 'conflict' : status === 400 || status === 422 ? 'invalid' : status === 401 || status === 403 ? 'rejected' : 'unavailable');
}
/** Bind reads and writes to the credential and account admitted for this operation. */
export function apiProfileEditing(baseURL: string, token: string | null, owner: string, rejected?: (token: string | null) => void, signal?: AbortSignal): ProfileEditingRepository {
  const api = createSessionApiClient(baseURL, () => token, signal, rejected);
  return {
    async read() {
      const result = await api.ownProfile();
      if (!result.response.ok || !result.data) throw failure(result.response.status, result.error?.code);
      return editableProfileFromAPI(result.data.data, owner);
    },
    async save(value, key) {
      const profile = normalizeEditableProfile(value);
      if (profile.userId !== owner) throw new ProfileEditFailure('rejected');
      const result = await api.updateOwnProfile({ username: profile.username, displayName: profile.displayName, description: profile.description, expectedRevision: profile.revision }, key);
      if (!result.response.ok || !result.data) throw failure(result.response.status, result.error?.code);
      return editableProfileFromAPI(result.data.data, owner);
    },
  };
}
