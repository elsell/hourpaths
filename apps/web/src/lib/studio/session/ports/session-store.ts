import type { Session } from '../domain/session';
export interface SessionStore {
  initialize?(): Promise<void>;
  refreshExclusive?<T>(operation: () => Promise<T>): Promise<T>;
  read(): Session | null;
  write(session: Session, expectedToken: string): void | Promise<void>;
  clear(): void | Promise<void>;
  pause?(owner: string): void | Promise<void>;
  retainedOwner?(): string | null;
}
export interface SessionService {
  refresh(session: Session): Promise<Session>;
  revoke(session: Session): Promise<void>;
}
