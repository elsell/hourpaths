import { PlatformSymbol } from './platform-symbol';
import { mobileTheme } from './tokens';
export function CommentHeartIcon({ selected }: { selected: boolean }) {
  return <PlatformSymbol systemName={selected ? 'heart.fill' : 'heart'} size={20}
    color={selected ? mobileTheme.colors.accent : mobileTheme.colors.textMuted} />;
}
