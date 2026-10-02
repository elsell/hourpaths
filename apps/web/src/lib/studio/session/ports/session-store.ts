import type { Session } from '../domain/session';
export interface SessionStore {
  read(): Session | null;
  write(session: Session): void;
  clear(): void;
  pause?(owner: string): void;
  retainedOwner?(): string | null;
}
export interface SessionService {
  refresh(session: Session): Promise<Session>;
  revoke(session: Session): Promise<void>;
}
