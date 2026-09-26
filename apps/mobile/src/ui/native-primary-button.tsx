import { NativeButton } from './native-button';
import type { NativeButtonProps } from './native-button.types';

export function NativePrimaryButton({ variant = 'prominent', ...props }: Omit<NativeButtonProps, 'variant'> & { variant?: 'plain' | 'prominent' }) {
  return <NativeButton {...props} variant={variant === 'plain' ? 'quiet' : 'primary'} />;
}
