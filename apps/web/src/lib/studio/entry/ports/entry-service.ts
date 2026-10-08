import type { EntryActivation, EntryContext, EntryReview, EntryPhase } from '../domain/entry';
export interface EntryService {
  restore(): Promise<EntryContext>;
  callback(): Promise<EntryContext>;
  review(): Promise<EntryReview>;
  defaults(): { timeZone: string; firstDayOfWeek: number };
  activate(input: EntryActivation): Promise<EntryContext>;
  recover(): Promise<void>;
  decline(): Promise<EntryContext>;
  begin(): Promise<void>;
  signOut(): Promise<void>;
  current(): boolean;
  expire(): void;
  navigate(destination: EntryPhase): void;
  openPolicy(url: string): void;
  dispose(): void;
}
