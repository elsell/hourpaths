import { createSessionApiClient, type ActiveFollowingItem, type PracticeFeedItem, type PublicProfile, type PracticeCommentItem } from '@hourpaths/api-client';
import type { ActivePerson, Comment, Page, Person, Profile, SocialEvent } from '../domain/activity';
import type { SocialRepository } from '../ports/social-repository';

export class SocialRequestError extends Error {
  constructor(readonly status: number) { super('social_request_failed'); }
}
function instant(value: string): number {
  const result = Date.parse(value);
  if (!Number.isFinite(result)) throw new SocialRequestError(502);
  return result;
}
function count(value: number): number {
  if (!Number.isSafeInteger(value) || value < 0) throw new SocialRequestError(502);
  return value;
}
function picture(value?: string): string | null {
  if (!value) return null;
  try { const url = new URL(value); return url.protocol === 'https:' ? url.href : null; } catch { return null; }
}
function participant(dto: PracticeFeedItem['participant']): Person {
  if (!dto.userId || !dto.username) throw new SocialRequestError(502);
  return { id: dto.userId, username: dto.username, name: dto.displayName, picture: picture(dto.profilePictureURL) };
}
function profileFromAPI(dto: PublicProfile): Profile {
  if (!dto.id || !dto.username) throw new SocialRequestError(502);
  return { id: dto.id, username: dto.username, name: dto.displayName, picture: picture(dto.profilePictureUrl),
    description: dto.description ?? '', followers: count(dto.followerCount), following: count(dto.followingCount), relationship: dto.relationship };
}
export function eventFromAPI(dto: PracticeFeedItem): SocialEvent {
  if (!dto.id || !dto.path.id || (dto.type === 'practice_session' ? !dto.activity : !dto.achievement)) throw new SocialRequestError(502);
  return { id: dto.id, publishedAt: instant(dto.publishedAt), author: participant(dto.participant),
    pathId: dto.path.id, pathName: dto.path.name, kind: dto.type,
    seconds: count(dto.type === 'practice_session' ? dto.activity!.durationSeconds : dto.achievement!.targetSeconds),
    edited: dto.activity?.edited ?? false, activityId: dto.activity?.id ?? null, achievement: dto.achievement?.kind ?? null,
    comments: count(dto.commentCount), canComment: dto.commentsEnabled, canReact: dto.reactionsEnabled,
    reactions: dto.emojiReactions.map(value => ({ emoji: value.emoji, count: count(value.count), selected: value.reacted })) };
}
function activeFromAPI(dto: ActiveFollowingItem): ActivePerson {
  return { person: participant(dto.participant), paths: dto.timers.map(timer => {
    const interval = timer.progress?.interval;
    const goal = interval ? { savedSeconds: count(interval.recordedSeconds), targetSeconds: count(interval.targetSeconds), startsAt: instant(interval.startedAt), endsAt: instant(interval.endedAt), recurrence: interval.recurrence } : null;
    if (!timer.id || !timer.path.id || (goal && (goal.endsAt <= goal.startsAt || !goal.targetSeconds))) throw new SocialRequestError(502);
    return { id: timer.id, pathId: timer.path.id, pathName: timer.path.name, startedAt: instant(timer.startedAt),
      savedSeconds: count(timer.progress?.accumulatedSeconds ?? 0), goal, overallTarget: timer.progress?.overallTargetSeconds ?? null };
  }) };
}
function commentFromAPI(dto: PracticeCommentItem): Comment {
  return { id: dto.comment.id, author: profileFromAPI(dto.author), text: dto.comment.text,
    version: count(dto.comment.version), createdAt: instant(dto.comment.createdAt), edited: dto.comment.edited,
    hearts: count(dto.heartCount), hearted: dto.heartedByViewer };
}
export function apiSocialRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): SocialRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  const accepted = <T>(result: { response: Response; data?: { data: T } }): T => {
    if (!result.response.ok || !result.data) throw new SocialRequestError(result.response.status);
    return result.data.data;
  };
  const changed = (result: { response: Response }) => {
    if (!result.response.ok) throw new SocialRequestError(result.response.status);
  };
  const page = <T, U>(result: { response: Response; data?: { data: { items: T[] }; meta: { nextCursor?: string } } }, map: (item: T) => U): Page<U> => {
    const data = accepted(result);
    return { items: data.items.map(map), next: result.data!.meta.nextCursor || null };
  };
  return {
    async connections(username, direction, cursor, signal) {
      const api = client(signal);
      const result = await (direction === 'followers' ? api.profileFollowers(username, cursor) : api.profileFollowing(username, cursor));
      return { items: accepted(result).map(profileFromAPI), next: result.data!.meta.nextCursor || null };
    },
    async removeFollower(userId, key) { changed(await client().removeFollower(userId, key)); },
    async viewer(signal) { return accepted(await client(signal).profile()).id; },
    async feed(cursor, signal) { return page(await client(signal).socialFeed(cursor), eventFromAPI); },
    async activity(username, cursor, signal) { return page(await client(signal).profileActivity(username, cursor), eventFromAPI); },
    async active(cursor, signal) { return page(await client(signal).socialActiveFollowing(cursor), activeFromAPI); },
    async profile(username, signal) { return profileFromAPI(accepted(await client(signal).profileByUsername(username))); },
    async profilePaths(username, signal) { const value = accepted(await client(signal).profilePaths(username)); return { count: count(value.pathCount), active: activeFromAPI(value.active) }; },
    async search(query, cursor, signal) { const result = await client(signal).searchProfiles(query, cursor); return { items: accepted(result).map(profileFromAPI), next: result.data!.meta.nextCursor || null }; },
    async relationship(profile, id) {
      if (profile.relationship === 'self') return;
      changed(await (profile.relationship === 'following' ? client().unfollowProfile(profile.username, id) : profile.relationship === 'requested' ? client().cancelFollowRequest(profile.username, id) : client().followProfile(profile.username, id)));
    },
    async reactions(eventId, emoji, selected, id) { changed(await (selected ? client().removeSocialFeedEmojiReaction(eventId, emoji, id) : client().addSocialFeedEmojiReaction(eventId, emoji, id))); },
    async reactionPeople(eventId, emoji, cursor, signal) { return page(await client(signal).socialFeedReactionPeople(eventId, emoji, cursor), profileFromAPI); },
    async comments(eventId, cursor, signal) { return page(await client(signal).socialFeedComments(eventId, cursor), commentFromAPI); },
    async addComment(eventId, text, id) { changed(await client().createSocialFeedComment(eventId, { text }, id)); },
    async editComment(eventId, comment, text, id) { changed(await client().editSocialFeedComment(eventId, comment.id, { text, expectedVersion: comment.version }, id)); },
    async deleteComment(eventId, commentId, id) { changed(await client().deleteSocialFeedComment(eventId, commentId, id)); },
    async commentHistory(eventId, commentId, cursor, signal) { const result = await client(signal).socialFeedCommentHistory(eventId, commentId, cursor); return { items: accepted(result).versions.map(value => ({ version: value.version, text: value.text, createdAt: instant(value.createdAt) })), next: result.data!.meta.nextCursor || null }; },
    async heart(eventId, comment, id) { changed(await (comment.hearted ? client().removePracticeCommentHeart(eventId, comment.id, id) : client().setPracticeCommentHeart(eventId, comment.id, id))); },
    async heartPeople(eventId, commentId, cursor, signal) { return page(await client(signal).practiceCommentHearts(eventId, commentId, cursor), profileFromAPI); },
    async requests(cursor, signal) { const result = await client(signal).followRequests(cursor); return { items: accepted(result).map(value => ({ id: value.id, person: profileFromAPI(value.requester) })), next: result.data!.meta.nextCursor || null }; },
    async answerRequest(id, accept, operationId) { changed(await (accept ? client().acceptFollowRequest(id, operationId) : client().rejectFollowRequest(id, operationId))); },
  };
}
