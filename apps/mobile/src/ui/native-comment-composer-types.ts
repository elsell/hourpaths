import type { NativeCommentInputProps } from './native-comment-input-types';

export type NativeCommentComposerProps = NativeCommentInputProps & {
  busy: boolean;
  disabled: boolean;
  label: string;
  onPress: () => void;
  stacked?: boolean;
};
