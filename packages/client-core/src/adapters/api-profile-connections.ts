import { createSessionApiClient, type PublicProfile } from '@hourpaths/api-client';
import { ProfileConnectionsFailure, type ConnectionPerson, type ProfileConnectionsRepository } from '../profile-connections';
function person(value: PublicProfile): ConnectionPerson {
  if (!value.id || !value.username || !value.displayName) throw new ProfileConnectionsFailure('unavailable');
  let picture: string | null = null;
  if (value.profilePictureUrl) {
    try { const url = new URL(value.profilePictureUrl); if (url.protocol === 'https:' && !url.username && !url.password) picture = url.href; } catch { /* Neutral avatar for unusable URLs. */ }
  }
  return { id: value.id, username: value.username, name: value.displayName, picture };
}
function failed(status: number): ProfileConnectionsFailure {
  return new ProfileConnectionsFailure(status === 404 ? 'hidden' : status === 401 || status === 403 ? 'rejected' : status === 409 ? 'pending' : 'unavailable');
}
export function apiProfileConnections(baseURL: string, token: string | null, ownerId: string, rejected?: (token: string | null) => void): ProfileConnectionsRepository {
  return {
    async list(username, direction, cursor, signal) {
      const api = createSessionApiClient(baseURL, () => token, signal, rejected);
      const result = await (direction === 'followers' ? api.profileFollowers(username, cursor) : api.profileFollowing(username, cursor));
      if (!result.response.ok || !result.data) throw failed(result.response.status);
      return { items: result.data.data.map(person), next: result.data.meta.nextCursor || null };
    },
    async remove(userId, key) {
      if (!ownerId || userId === ownerId) throw new ProfileConnectionsFailure('rejected');
      const api = createSessionApiClient(baseURL, () => token, undefined, rejected);
      const result = await api.removeFollower(userId, key);
      if (!result.response.ok || !result.data) throw failed(result.response.status);
      if (result.data.data.profile.id !== userId) throw new ProfileConnectionsFailure('unavailable');
    },
  };
}
