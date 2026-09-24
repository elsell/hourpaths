export type CommentSubmissionOwner = Readonly<{
  active: () => boolean;
  admit: () => symbol | null;
  owns: (admission: symbol) => boolean;
  release: (admission: symbol) => boolean;
}>;

export function createCommentSubmissionOwner(): CommentSubmissionOwner {
  let current: symbol | null = null;
  return {
    active() { return current !== null; },
    admit() {
      if (current) return null;
      current = Symbol('comment-submission');
      return current;
    },
    owns(admission) { return current === admission; },
    release(admission) {
      if (current !== admission) return false;
      current = null;
      return true;
    },
  };
}
