export interface Person { id: string; username: string; name: string; picture: string | null }
export interface Profile extends Person {
  description: string; followers: number; following: number;
  relationship: 'self' | 'none' | 'requested' | 'following';
}
export interface SocialEvent {
  id: string; publishedAt: number; author: Person; pathId: string; pathName: string;
  kind: 'practice_session' | 'goal_achievement'; seconds: number; edited: boolean;
  achievement: 'interval' | 'overall' | null; activityId: string | null;
  comments: number; canComment: boolean; canReact: boolean;
  reactions: readonly { emoji: string; count: number; selected: boolean }[];
}
export interface LivePath {
  id: string; pathId: string; pathName: string; startedAt: number; savedSeconds: number;
  goal: { savedSeconds: number; targetSeconds: number; startsAt: number; endsAt: number; recurrence: string } | null;
  overallTarget: number | null;
}
export interface ActivePerson { person: Person; paths: readonly LivePath[] }
export interface Comment {
  id: string; author: Person; text: string; version: number; createdAt: number; edited: boolean;
  hearts: number; hearted: boolean;
}
export interface CommentRevision { version: number; text: string; createdAt: number }
export interface Page<T> { items: readonly T[]; next: string | null }
export interface FollowRequest { id: string; person: Profile }
export function chronologicalEvents(events: readonly SocialEvent[]): SocialEvent[] {
  return [...new Map(events.map(event => [event.id, event])).values()]
    .sort((a, b) => b.publishedAt - a.publishedAt || b.id.localeCompare(a.id));
}
export function adjacentPage(pages: readonly { id: string }[], selected: string, direction: -1 | 1): string | null {
  const index = pages.findIndex(page => page.id === selected);
  if (index < 0) return null;
  return pages[Math.max(0, index + direction)]?.id ?? null;
}
export function liveProgress(path: Pick<LivePath, 'startedAt' | 'savedSeconds' | 'goal'>, now: number) {
  const elapsed = (start: number, end: number) => Math.max(0, Math.floor((end - start) / 1000));
  const sessionSeconds = elapsed(path.startedAt, now);
  return {
    sessionSeconds, totalSeconds: path.savedSeconds + sessionSeconds,
    goalSeconds: path.goal ? path.goal.savedSeconds + elapsed(Math.max(path.startedAt, path.goal.startsAt), Math.min(now, path.goal.endsAt)) : null,
    goalExpired: path.goal ? now >= path.goal.endsAt : false,
  };
}
