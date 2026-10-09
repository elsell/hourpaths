import type { ConnectionDirection } from '@hourpaths/client-core';
import type { ActivePerson, Comment, CommentRevision, FollowRequest, Page, Person, Profile, SocialEvent } from '../domain/activity';
export interface SocialRepository {
  connections(username: string, direction: ConnectionDirection, cursor?: string, signal?: AbortSignal): Promise<Page<Person>>;
  removeFollower(userId: string, operationId: string): Promise<void>;
  feed(cursor?: string, signal?: AbortSignal): Promise<Page<SocialEvent>>;
  activity(username: string, cursor?: string, signal?: AbortSignal): Promise<Page<SocialEvent>>;
  active(cursor?: string, signal?: AbortSignal): Promise<Page<ActivePerson>>;
  profile(username: string, signal?: AbortSignal): Promise<Profile>;
  profilePaths(username: string, signal?: AbortSignal): Promise<{ count: number; active: ActivePerson }>;
  search(query: string, cursor?: string, signal?: AbortSignal): Promise<Page<Profile>>;
  relationship(profile: Profile, operationId: string): Promise<void>;
  reactions(eventId: string, emoji: string, selected: boolean, operationId: string): Promise<void>;
  reactionPeople(eventId: string, emoji: string, cursor?: string, signal?: AbortSignal): Promise<Page<Person>>;
  comments(eventId: string, cursor?: string, signal?: AbortSignal): Promise<Page<Comment>>;
  addComment(eventId: string, text: string, operationId: string): Promise<void>;
  editComment(eventId: string, comment: Comment, text: string, operationId: string): Promise<void>;
  deleteComment(eventId: string, commentId: string, operationId: string): Promise<void>;
  commentHistory(eventId: string, commentId: string, cursor?: string, signal?: AbortSignal): Promise<Page<CommentRevision>>;
  heart(eventId: string, comment: Comment, operationId: string): Promise<void>;
  heartPeople(eventId: string, commentId: string, cursor?: string, signal?: AbortSignal): Promise<Page<Person>>;
  requests(cursor?: string, signal?: AbortSignal): Promise<Page<FollowRequest>>;
  answerRequest(id: string, accept: boolean, operationId: string): Promise<void>;
  viewer(signal?: AbortSignal): Promise<string>;
}
