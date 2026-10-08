export interface AccountDeletionService {
  review(): Promise<{ owner: string; name: string }>;
  confirm(owner: string): Promise<void>;
  resume(owner: string): Promise<void>;
  pendingOwners(): Promise<string[]>;
}
