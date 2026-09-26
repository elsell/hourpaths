const localDatePattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const localTimePattern = /^(\d{2}):(\d{2})(?::(\d{2}))?$/;

function pad(value: number): string {
  return String(value).padStart(2, '0');
}

export function manualOccurrencePickerValue(localDate: string, localTime: string): Date | null {
  const dateMatch = localDatePattern.exec(localDate);
  const timeMatch = localTimePattern.exec(localTime);
  if (!dateMatch || !timeMatch) return null;

  const year = Number(dateMatch[1]);
  const month = Number(dateMatch[2]);
  const day = Number(dateMatch[3]);
  const hour = Number(timeMatch[1]);
  const minute = Number(timeMatch[2]);
  const second = Number(timeMatch[3] ?? 0);
  if (month < 1 || month > 12 || day < 1 || day > 31 ||
    hour > 23 || minute > 59 || second > 59) return null;

  const value = new Date(Date.UTC(year, month - 1, day, hour, minute, second));
  if (value.getUTCFullYear() !== year || value.getUTCMonth() !== month - 1 || value.getUTCDate() !== day) {
    return null;
  }
  return value;
}

export function localDateFromPicker(value: Date): string {
  return `${value.getUTCFullYear()}-${pad(value.getUTCMonth() + 1)}-${pad(value.getUTCDate())}`;
}

export function localTimeFromPicker(value: Date): string {
  return `${pad(value.getUTCHours())}:${pad(value.getUTCMinutes())}:00`;
}

// Compose's clock reads a device-local Calendar. Anchor only its clock fields
// on a stable date; the activity date and participant timezone remain untouched.
export function androidClockPickerValue(localTime: string): Date | null {
  const value = manualOccurrencePickerValue('2000-01-15', localTime);
  return value ? new Date(2000, 0, 15, value.getUTCHours(), value.getUTCMinutes(), value.getUTCSeconds()) : null;
}

export function localTimeFromAndroidClock(value: Date): string {
  return `${pad(value.getHours())}:${pad(value.getMinutes())}:00`;
}
