import type { MessageKey } from '@hourpaths/i18n';
import type { PracticeReaction } from '@hourpaths/api-client';

export type SocialReaction = PracticeReaction;

export type SocialReactionCounts = Record<SocialReaction, number>;

export type SocialReactionDefinition = {
  emoji: string;
  labelKey: MessageKey;
  type: SocialReaction;
};

export type SocialReactionMenuChoice = {
  emoji: string;
  label: string;
  menuLabel: string;
  onPress: () => void;
  selected: boolean;
  type: SocialReaction;
};

export const SOCIAL_REACTIONS = [
  { emoji: '❤️', labelKey: 'social.reactionHeart', type: 'heart' },
  { emoji: '👏', labelKey: 'social.reactionApplause', type: 'applause' },
  { emoji: '🔥', labelKey: 'social.reactionFire', type: 'fire' },
  { emoji: '💪', labelKey: 'social.reactionStrong', type: 'strong' },
  { emoji: '🎉', labelKey: 'social.reactionCelebrate', type: 'celebrate' },
] as const satisfies readonly SocialReactionDefinition[];

export function visibleReactionCounts(counts: SocialReactionCounts) {
  return SOCIAL_REACTIONS
    .filter(({ type }) => counts[type] > 0)
    .map((reaction) => ({ ...reaction, count: counts[reaction.type] }));
}

export function socialReactionDefinition(type: SocialReaction) {
  return SOCIAL_REACTIONS.find((reaction) => reaction.type === type)!;
}

export function socialReactionFromAPI(value: string | null): SocialReaction | null {
  return SOCIAL_REACTIONS.some(({ type }) => type === value) ? value as SocialReaction : null;
}
