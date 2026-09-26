export type NativeCommentInputProps = {
  accessibilityLabel: string;
  autoFocus?: boolean;
  editable?: boolean;
  maximumLines?: number;
  minimumLines?: number;
  onChangeText: (value: string) => void;
  placeholder?: string;
  value: string;
};
