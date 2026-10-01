export interface LeaveReview { readonly pathId: string; readonly name: string; readonly participant: boolean; readonly retainActivity?: boolean }
export type LeaveResult = { kind: 'applied' } | { kind: 'failed' } | { kind: 'superseded' };
