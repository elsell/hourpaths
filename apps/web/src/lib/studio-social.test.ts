import test from 'node:test';
import assert from 'node:assert/strict';
import { liveProgress, adjacentPage, chronologicalEvents } from './studio/social/domain/activity';
import { eventFromAPI } from './studio/social/adapters/api-social-repository';

test('live progress counts only overlap and identifies expired goal periods', () => {
  const page = { id: 't', pathId: 'p', pathName: 'guitar', startedAt: 5000, savedSeconds: 20,
    goal: { savedSeconds: 3, targetSeconds: 60, startsAt: 10000, endsAt: 20000, recurrence: 'hourly' } };
  assert.deepEqual(liveProgress(page, 25000), { sessionSeconds: 20, totalSeconds: 40, goalSeconds: 13, goalExpired: true });
  assert.equal(adjacentPage([{ id: 'a' }, { id: 'b' }], 'gone', 1), null);
  assert.equal(adjacentPage([{ id: 'a' }, { id: 'b' }], 'b', 1), null);
});

test('social mapping preserves publication chronology and independent emoji selections without leaking DTOs', () => {
  const dto = { id: 'e', type: 'practice_session' as const, publishedAt: '2026-09-29T12:00:00Z',
    participant: { userId: 'u', username: 'john', displayName: 'john' }, path: { id: 'p', name: 'guitar' },
    activity: { id: 'a', durationSeconds: 60, edited: true }, reactions: { heart: 0, applause: 0, fire: 0, strong: 0, celebrate: 0 },
    viewerReaction: null, commentCount: 2, commentsEnabled: true, reactionsEnabled: true,
    emojiReactions: [{ emoji: '🎸', count: 2, reacted: true }, { emoji: '👏', count: 1, reacted: true }] };
  const mapped = eventFromAPI(dto);
  assert.equal(mapped.publishedAt, Date.parse(dto.publishedAt));
  assert.equal(mapped.seconds, 60);
  assert.equal(mapped.comments, 2);
  assert.equal(mapped.reactions.filter(r => r.selected).length, 2);
  dto.emojiReactions[0].count = 99;
  assert.equal(mapped.reactions[0].count, 2);
  assert.equal('activity' in mapped, false);
  assert.deepEqual(chronologicalEvents([mapped, { ...mapped, id: 'old', publishedAt: 1 }, mapped]).map(e => e.id), ['e', 'old']);
  assert.throws(() => eventFromAPI({ ...dto, publishedAt: 'invalid' }));
});
