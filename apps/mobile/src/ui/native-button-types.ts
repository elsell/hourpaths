export type NativeButtonProps = {
  accessibilityLabel?: string;
  busy?: boolean;
  disabled?: boolean;
  fullWidth?: boolean;
  label: string;
  tone?: { background: string; foreground: string };
  onPress: () => void;
  selected?: boolean;
  systemImage?: string;
  variant?: 'primary' | 'secondary' | 'quiet' | 'danger';
};
