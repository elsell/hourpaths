import type { ActivityInput, ActivityWrite } from '../domain/write';
import type { ActivityRepository } from '../ports/activity-repository';
export class ActivityInputInvalid extends Error {
  constructor(readonly field: 'note' | 'duration') { super('activity_input_invalid'); }
}
export function reviewActivityWrite(pathId: string, activityId: string | null, input: ActivityInput, operationId: string): ActivityWrite {
  const normalized = input.note.normalize('NFC');
  const note = normalized.trim() ? normalized : '';
  if (!pathId || !operationId || !Number.isSafeInteger(input.seconds) || input.seconds <= 0) throw new ActivityInputInvalid('duration');
  if ([...note].length > 2000 || /[\u0000-\u0009\u000b\u000c\u000e-\u001f\u007f-\u009f]/u.test(note)) throw new ActivityInputInvalid('note');
  return Object.freeze({ pathId, activityId, operationId, input: Object.freeze({ ...input, note }) });
}
export function saveReviewedActivity(repository: Pick<ActivityRepository, 'save'>, review: ActivityWrite) { return repository.save(review); }
