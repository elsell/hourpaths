import { NativeButton } from './native-button';
import type { NativeButtonProps } from './native-button-types';

export function NativePrimaryButton({ accessibilityLabel, busy, disabled, fullWidth, label, onPress, selected, systemImage, variant = 'prominent' }: Omit<NativeButtonProps, 'variant'> & { variant?: 'plain' | 'prominent' }) {
  return <NativeButton accessibilityLabel={accessibilityLabel} busy={busy} disabled={disabled} fullWidth={fullWidth} label={label} onPress={onPress} selected={selected} systemImage={systemImage} variant={variant === 'plain' ? 'quiet' : 'primary'} />;
}
