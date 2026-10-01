import type { Visibility, VisibilityContext } from '../domain/visibility';
import type { VisibilityCommands } from '../ports/visibility-commands';
export function chooseVisibility(commands: VisibilityCommands, context: VisibilityContext, choice: Visibility) {
  const review = commands.review(context, choice);
  return review ? { kind: review.broader ? 'confirm' as const : 'submit' as const, review } : { kind: 'unchanged' as const };
}
