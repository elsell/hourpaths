import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createSocialSessionTarget,
  currentSocialSessionCredential,
  ownsSocialSessionTarget,
  rotateSocialSessionTarget,
  socialRouteHref,
  socialRouteIntent,
  queueSocialRouteBootstrap,
  takeSocialRouteBootstrap,
} from './social-route-recovery';

test('admits only exact singleton social routes', () => {
  assert.deepEqual(socialRouteIntent('/following', {}), { kind: 'following', routeKey: 'social:following' });
  assert.deepEqual(socialRouteIntent('/following/people', {}), { kind: 'people', routeKey: 'social:people' });
  assert.deepEqual(socialRouteIntent('/follow-requests', {}), {
    kind: 'follow-requests',
    routeKey: 'social:follow-requests',
  });
  assert.deepEqual(socialRouteIntent('/profile/developer', { username: 'developer' }), {
    kind: 'profile',
    routeKey: 'social:profile:developer',
    username: 'developer',
  });
  assert.deepEqual(socialRouteIntent('/following/activity/path-1/activity-1', { pathID: 'path-1', activityID: 'activity-1' }), {
    activityID: 'activity-1', kind: 'activity', pathID: 'path-1', routeKey: 'social:activity:path-1:activity-1',
  });
  assert.deepEqual(socialRouteIntent('/following/comments/event-1', { eventID: 'event-1' }), {
    eventID: 'event-1', kind: 'comments', routeKey: 'social:comments:event-1',
  });
  assert.deepEqual(socialRouteIntent('/following/comments/event-1/hearts/comment-1', { eventID: 'event-1', commentID: 'comment-1' }), {
    commentID: 'comment-1', eventID: 'event-1', kind: 'comment-hearts', routeKey: 'social:comment-hearts:event-1:comment-1',
  });

  assert.equal(socialRouteIntent('/profile/developer', { username: ['developer'] }), null);
  assert.equal(socialRouteIntent('/profile/developer', { username: 'someone-else' }), null);
  assert.equal(socialRouteIntent('/following/comments/event', { eventID: ['event'] }), null);
  assert.equal(socialRouteIntent('/profile/', { username: '' }), null);
});

test('cold social bootstrap retains the exact latest destination until Home adopts it', () => {
  const comments = socialRouteIntent('/following/comments/event-1', { eventID: 'event-1' });
  const profile = socialRouteIntent('/profile/developer', { username: 'developer' });
  assert.ok(comments);
  assert.ok(profile);
  queueSocialRouteBootstrap(comments);
  queueSocialRouteBootstrap(profile);
  assert.equal(socialRouteHref(profile), '/profile/developer');
  assert.deepEqual(takeSocialRouteBootstrap(), profile);
  assert.equal(takeSocialRouteBootstrap(), null);
});

test('retains arbitrary same-owner credential rotations without admitting replacements', () => {
  const sessionA = { token: 'token-a' };
  const sessionB = { token: 'token-b' };
  const sessionC = { token: 'token-c' };
  const initial = createSocialSessionTarget('owner-a', sessionA, 'social:profile:developer');
  const rotatedB = rotateSocialSessionTarget(initial, 'owner-a', sessionB);
  const rotatedC = rotateSocialSessionTarget(rotatedB, 'owner-a', sessionC);

  assert.equal(ownsSocialSessionTarget(rotatedC, 'owner-a', 'token-a', 'social:profile:developer'), true);
  assert.equal(ownsSocialSessionTarget(rotatedC, 'owner-a', 'token-b', 'social:profile:developer'), true);
  assert.equal(ownsSocialSessionTarget(rotatedC, 'owner-a', 'token-c', 'social:profile:developer'), true);
  assert.equal(ownsSocialSessionTarget(rotatedC, 'owner-b', 'token-c', 'social:profile:developer'), false);
  assert.equal(ownsSocialSessionTarget(rotatedC, 'owner-a', 'token-c', 'social:profile:other'), false);
  assert.strictEqual(rotateSocialSessionTarget(rotatedC, 'owner-b', { token: 'replacement' }), rotatedC);
  assert.strictEqual(currentSocialSessionCredential(rotatedC, 'owner-a', 'social:profile:developer', sessionC), sessionC);
  assert.equal(currentSocialSessionCredential(rotatedC, 'owner-b', 'social:profile:developer', sessionC), null);
});
