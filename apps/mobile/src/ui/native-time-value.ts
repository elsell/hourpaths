// DatePicker transports wall-clock fields through a neutral calendar date.
export function clockMinuteValue(minute: number): string {
  return `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`;
}
