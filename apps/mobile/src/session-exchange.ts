import type { SessionExchangeCredential } from '@hourpaths/client-core';

export type ExchangedSession = SessionExchangeCredential;
export type SessionRecoveryOperation = 'refresh' | 'profile';

export type MobileSessionDestination<HomeProfile, OnboardingProfile> =
  | { kind: 'home'; profile: HomeProfile }
  | { kind: 'onboarding'; profile: OnboardingProfile }
  | { kind: 'duplicate_email_recovery'; profile: OnboardingProfile };

type ExchangeActivationDependencies = {
  credential: ExchangedSession;
  activate(credential: ExchangedSession): Promise<void>;
  handleFailure(cause: unknown, credential: ExchangedSession): Promise<void>;
};

export async function activateExchangedSession(
  dependencies: ExchangeActivationDependencies,
): Promise<void> {
  try {
    await dependencies.activate(dependencies.credential);
  } catch (cause) {
    await dependencies.handleFailure(cause, dependencies.credential);
  }
}

type PersistedActivationDependencies<HomeProfile, OnboardingProfile> = {
  credential: ExchangedSession;
  renewable: boolean;
  current(): boolean;
  adopt(credential: ExchangedSession, renewable: boolean): void;
  loadHome(credential: ExchangedSession): Promise<HomeProfile>;
  loadOnboarding(credential: ExchangedSession): Promise<OnboardingProfile>;
  online(destination: MobileSessionDestination<HomeProfile, OnboardingProfile>): void;
};

export async function activatePersistedMobileSession<HomeProfile, OnboardingProfile>(
  dependencies: PersistedActivationDependencies<HomeProfile, OnboardingProfile>,
): Promise<void> {
  if (!dependencies.current()) return;
  dependencies.adopt(
    dependencies.credential,
    dependencies.renewable && dependencies.credential.nextAction === 'home',
  );
  const destination = dependencies.credential.nextAction === 'home'
    ? { kind: 'home' as const, profile: await dependencies.loadHome(dependencies.credential) }
    : {
      kind: dependencies.credential.nextAction,
      profile: await dependencies.loadOnboarding(dependencies.credential),
    };
  if (dependencies.current()) dependencies.online(destination);
}

type MobileExchangeDependencies = {
  exchange(): Promise<ExchangedSession>;
  current(): boolean;
  acknowledge(): void;
  activate(credential: ExchangedSession, renewable: boolean): Promise<void>;
  handleFailure(
    cause: unknown,
    credential: ExchangedSession,
    operation: SessionRecoveryOperation,
  ): Promise<void>;
  revokeSuperseded(credential: ExchangedSession): Promise<void>;
};

export async function completeMobileSessionExchange(
  dependencies: MobileExchangeDependencies,
): Promise<void> {
  const credential = await dependencies.exchange();
  dependencies.acknowledge();
  if (!dependencies.current()) {
    await dependencies.revokeSuperseded(credential);
    return;
  }
  await activateExchangedSession({
    credential,
    activate: (current) => dependencies.activate(current, true),
    handleFailure: (cause, current) => dependencies.handleFailure(cause, current, 'profile'),
  });
}

type MobileRecoveryDependencies = {
  mode: SessionRecoveryOperation;
  credential: ExchangedSession;
  renewable: boolean;
  current(): boolean;
  refresh(credential: ExchangedSession): Promise<ExchangedSession>;
  expiryAdvanced?(previous: string, next: string): boolean;
  activate(credential: ExchangedSession, renewable: boolean): Promise<void>;
  handleFailure(
    cause: unknown,
    credential: ExchangedSession,
    operation: SessionRecoveryOperation,
  ): Promise<void>;
  revokeSuperseded(credential: ExchangedSession): Promise<void>;
};

export async function recoverMobileSession(
  dependencies: MobileRecoveryDependencies,
): Promise<void> {
  if (!dependencies.current()) return;
  let credential = dependencies.credential;
  let renewable = dependencies.renewable;
  if (dependencies.mode === 'refresh') {
    try {
      const previousExpiry = credential.expiresAt;
      credential = await dependencies.refresh(credential);
      if (!dependencies.current()) {
        await dependencies.revokeSuperseded(credential);
        return;
      }
      renewable = dependencies.expiryAdvanced
        ? dependencies.expiryAdvanced(previousExpiry, credential.expiresAt)
        : renewable;
    } catch (cause) {
      if (dependencies.current()) await dependencies.handleFailure(cause, credential, 'refresh');
      return;
    }
  }
  try { await dependencies.activate(credential, renewable); }
  catch (cause) {
    if (dependencies.current()) await dependencies.handleFailure(cause, credential, 'profile');
  }
}
