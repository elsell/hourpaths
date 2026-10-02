import type { MobileHomeProfile } from '../session-destination';

export type RetainedMobileHome = { owner: string; timeZone: string; profile: MobileHomeProfile };
export interface MobileHomeCache {
  readHome(owner: string): Promise<RetainedMobileHome | null>;
  saveHome(value: RetainedMobileHome): Promise<void>;
}
