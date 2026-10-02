export interface ClockCorrectionView {
  reviewedStartedAt?: string;
  id: string; pathName: string; startedAt: string; endedAt: string; timeZone: string;
}

interface TrackingNoticeView {
  id: string;
  reason: 'membership' | 'deleted' | 'conflict' | 'archived' | 'validation';
  pathName?: string;
  savedSeconds?: number;
  discardedSeconds?: number;
}

export interface TrackingStatus {
  offline: boolean;
  showBanner: boolean;
  pending: boolean;
  unavailablePathIds?: readonly string[];
  notices: TrackingNoticeView[];
  corrections: ClockCorrectionView[];
}
export interface OfflineStatus {
  subscribe(listener: () => void): () => void;
  snapshot(): Promise<TrackingStatus>;
  dismissBanner(): void;
  dismissNotice(id: string): Promise<void>;
  retry(): void;
  correct(id: string, start: string, end: string): Promise<void>;
}
