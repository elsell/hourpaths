export const defaultOnboardingVisibility = 'public' as const;

/** Presentation-independent navigation. Activation remains owned by the session operation. */
export const onboardingSteps = ['identity', 'preferences', 'review', 'photo'] as const;
export type OnboardingStep = typeof onboardingSteps[number];

export function advanceOnboardingStep(step: OnboardingStep, valid: boolean, busy: boolean): OnboardingStep {
  if (busy || !valid || step === 'review' || step === 'photo') return step;
  return onboardingSteps[Math.min(onboardingSteps.indexOf(step) + 1, onboardingSteps.length - 1)];
}

export function previousOnboardingStep(step: OnboardingStep, busy: boolean): OnboardingStep {
  if (busy || step === 'photo') return step;
  return onboardingSteps[Math.max(onboardingSteps.indexOf(step) - 1, 0)];
}

export function recoverOnboardingStep(step: OnboardingStep, failure: string | undefined): OnboardingStep {
  switch (failure) {
    case 'username': return 'identity';
    case 'policy': return 'review';
    case 'timeZone':
    case 'locale': return 'preferences';
    default: return step;
  }
}
