import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const route = readFileSync(new URL('../routes/+page.svelte', import.meta.url), 'utf8');

test('authenticated people discovery is a directly reachable primary web surface', () => {
  assert.match(route, /import SocialProfileDiscovery from '\$lib\/social-profile-discovery\.svelte'/);
  assert.match(route, /primarySurface === 'home'/);
  assert.match(route, /primarySurface === 'following'/);
  assert.match(route, /i18n\.t\('home\.heading'\)/);
  assert.match(route, /i18n\.t\('social\.following'\)/);
  assert.match(route, /<SocialProfileDiscovery\b/);
});

test('the route delegates validated, session-owned search and profile reads', () => {
  assert.match(route, /createProfileSearchOwner\(\)/);
  assert.match(route, /profileSearchQuery\(/);
  assert.match(route, /profileSearchPageFromAPI\(/);
  assert.match(route, /publicProfileFromAPI\(/);
  assert.match(route, /\.searchProfiles\(/);
  assert.match(route, /\.profileByUsername\(/);
  assert.match(route, /socialProfileSearchOperations\.issue\(\)/);
  assert.match(route, /socialProfileDetailOperations\.issue\(\)/);
  assert.match(route, /handleFailure\(cause, current, current\.expiresAt, 'profile', ticket\)/);
  assert.match(route, /resetSocialProfileDiscovery\(\)/);
});

test('the route wires authoritative follow mutations and a dedicated request destination without remote image rendering', () => {
  assert.match(route, /relationshipMutationResultFromAPI/);
  assert.match(route, /\.followProfile\(username, idempotencyKey\)/);
  assert.match(route, /\.cancelFollowRequest\(username, idempotencyKey\)/);
  assert.match(route, /\.unfollowProfile\(username, idempotencyKey\)/);
  assert.match(route, /followRequestPageFromAPI/);
  assert.match(route, /followRequestReviewResultFromAPI/);
  assert.match(route, /\.followRequests\(cursor \|\| undefined\)/);
  assert.match(route, /\.acceptFollowRequest\(requestID, key\)/);
  assert.match(route, /\.rejectFollowRequest\(requestID, key\)/);
  assert.doesNotMatch(route, /profilePictureUrl[^\n]*(?:src=|<img)/);
});
