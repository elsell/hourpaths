import type { ProfilePictureRepository } from '@hourpaths/client-core';
import { onboardingSteps } from '@hourpaths/client-core';
import type { Translator } from '@hourpaths/i18n';
import { NativeSheet, ThemedText } from './primitives';
import { PictureSettings } from './picture-settings';

/** Uses the active account's existing image boundary; never activates an account. */
export function OnboardingPhoto({ repository, pick, operationId, isCurrent, i18n, onFinish }: {
  repository: ProfilePictureRepository;
  pick(): Promise<string | null>;
  operationId(): string;
  isCurrent(): boolean;
  i18n: Translator;
  onFinish(): void;
}) {
  return <NativeSheet visible compact dismissible={false} presentationStyle="fullScreen"
    title={i18n.t('onboarding.photoHeading')}
    onRequestClose={onFinish}
    trailingAction={{ label: i18n.t('onboarding.skipPhoto'), onPress: onFinish }}>
    <ThemedText>{i18n.t('onboarding.stepProgress', { current: onboardingSteps.length, total: onboardingSteps.length })}</ThemedText>
    <ThemedText>{i18n.t('onboarding.photoBody')}</ThemedText>
    <PictureSettings repository={repository} pick={pick} operationId={operationId} isCurrent={isCurrent} i18n={i18n} onSaved={onFinish} />
  </NativeSheet>;
}
