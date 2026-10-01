import type { TransferCommand } from '../domain/transfer';
import type { OwnershipRepository } from '../ports/ownership-repository';
export function ownershipCommands(repository: OwnershipRepository, newKey: () => string) {
  let disposed = false, pending = false;
  let attempt: { identity: string; key: string; command: TransferCommand } | null = null;
  return {
    clear() { if (!pending) attempt = null; },
    dispose() { disposed = true; attempt = null; },
    async submit(command: TransferCommand) {
      if (disposed || pending) return null;
      const identity = command.kind === 'initiate' ? `${command.pathId}:${command.review.reservationToken}` : `${command.kind}:${command.transfer.id}`;
      if (!attempt || attempt.identity !== identity) attempt = { identity, key: newKey(), command };
      const admitted = attempt;
      pending = true;
      try {
        const result = await repository.execute(admitted.command, admitted.key);
        return disposed ? null : result;
      } finally { pending = false; }
    },
  };
}
