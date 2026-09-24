import { Alert } from 'react-native';

export function presentNativeDestructiveConfirmation({
  title,
  message,
  cancelLabel,
  confirmLabel,
  onCancel,
  onConfirm,
}: {
  title: string;
  message: string;
  cancelLabel: string;
  confirmLabel: string;
  onCancel?: () => void;
  onConfirm: () => void;
}) {
  Alert.alert(title, message, [
    { text: cancelLabel, style: 'cancel', onPress: onCancel },
    { text: confirmLabel, style: 'destructive', onPress: onConfirm },
  ]);
}

export function presentNativePathLeaveChoice({
  cancelLabel,
  deleteLabel,
  keepLabel,
  message,
  onCancel,
  onDelete,
  onKeep,
  title,
}: {
  cancelLabel: string;
  deleteLabel: string;
  keepLabel: string;
  message: string;
  onCancel: () => void;
  onDelete: () => void;
  onKeep: () => void;
  title: string;
}) {
  Alert.alert(title, message, [
    { onPress: onKeep, style: 'default', text: keepLabel },
    { onPress: onDelete, style: 'destructive', text: deleteLabel },
    { onPress: onCancel, style: 'cancel', text: cancelLabel },
  ]);
}
