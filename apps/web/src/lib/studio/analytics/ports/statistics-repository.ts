import type { Selection, Statistics } from '../domain/statistics';
export interface StatisticsRepository { load(selection: Selection, signal?: AbortSignal): Promise<Statistics> }
