// One pictographic emoji (including skin-tone/ZWJ/tag sequences), flag, or
// keycap. Accept native keyboard emoji without accepting arbitrary Path copy.
const emoji = /^(?:\p{Regional_Indicator}{2}|[#*0-9]\uFE0F?\u20E3|\p{Extended_Pictographic}\uFE0F?\p{Emoji_Modifier}?(?:[\u{E0020}-\u{E007E}]+\u{E007F})?(?:\u200D\p{Extended_Pictographic}\uFE0F?\p{Emoji_Modifier}?)*)$/u;
export function validPathEmoji(value: string): boolean {
  return value.length <= 64 && emoji.test(value);
}
