import type { ProfileVisibility } from './path-visibility';

export type AuthenticatedProfile = Readonly<{
  id: string;
  email: string;
  displayName: string;
  profileVisibility: ProfileVisibility;
}>;

function validText(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value &&
    !/[\u0000-\u001f\u007f]/u.test(value);
}

export function authenticatedProfileFromAPI(value: unknown): AuthenticatedProfile {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid authenticated profile');
  }
  const record = value as Record<string, unknown>;
  if (
    Object.keys(record).sort().join(',') !== 'displayName,email,id,profileVisibility' ||
    !validText(record.id) ||
    !validText(record.email) ||
    !validText(record.displayName) ||
    (record.profileVisibility !== 'private' && record.profileVisibility !== 'public')
  ) throw new Error('invalid authenticated profile');
  return Object.freeze({
    id: record.id,
    email: record.email,
    displayName: record.displayName,
    profileVisibility: record.profileVisibility,
  });
}
