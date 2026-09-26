export type NativeMenuAction = {
  destructive?: boolean;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  systemImage?: string;
};
