export interface PathStatistics {
  totalSeconds: number;
  sessionCount: number;
  averageSeconds: number;
  weekStartsOn: number;
  firstDate: string;
  lastDate: string;
  days: readonly { date: string; seconds: number }[];
}
