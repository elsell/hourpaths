import { declineDuplicateEmailRecovery, type SessionExchangeCredential } from '@hourpaths/client-core';

type ContinueMobileNewAccountDependencies = {
  credential: SessionExchangeCredential;
  current(): boolean;
  decline(): Promise<void>;
  persist(credential: SessionExchangeCredential): Promise<void>;
  activate(credential: SessionExchangeCredential): Promise<void>;
};

export async function continueMobileNewAccount(
  dependencies: ContinueMobileNewAccountDependencies,
): Promise<SessionExchangeCredential | null> {
  const replacement = declineDuplicateEmailRecovery(dependencies.credential);
  if (!dependencies.current()) return null;
  await dependencies.decline();
  if (!dependencies.current()) return null;
  await dependencies.persist(replacement);
  if (!dependencies.current()) return null;
  await dependencies.activate(replacement);
  return replacement;
}
