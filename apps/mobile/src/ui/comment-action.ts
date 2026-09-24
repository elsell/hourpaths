export type CommentAction = {
  destructive?: boolean;
  disabled?: boolean;
  label: string;
  onPress: () => void;
  systemImage: string;
};
