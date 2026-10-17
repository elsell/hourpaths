import { getLocales } from 'expo-localization';
import { useEffect, useRef, useState, type ElementRef, type ReactNode } from 'react';
import { advanceOnboardingStep, previousOnboardingStep, onboardingSteps, type OnboardingStep } from '@hourpaths/client-core';
import { StyleSheet, Switch, View, useWindowDimensions } from 'react-native';
import { createDeviceTranslator } from '../i18n';
import {
  validProfileDisplayName,
  validProfileUsername,
  type MobileOnboardingDraft,
} from '../session-destination';
import {
  ActionButton,
  NativeSheet,
  SectionHeading,
  StatusBanner,
  Surface,
  ThemedText,
  ThemedTextInput,
} from './primitives';
import { needsCompactVerticalLayout } from './adaptive-layout';
import { mobileTheme } from './tokens';

const i18n = createDeviceTranslator(getLocales);

type OnboardingFormProps = {
  busy: boolean;
  canSubmit: boolean;
  errorText?: string;
  homeRecoveryStatus?: 'loading' | 'offline' | 'error';
  onChangeDisplayName: (displayName: string) => void;
  onChangeUsername: (username: string) => void;
  onConfirm: () => void;
  onOpenPolicy: (url: string) => void;
  onReviewUsername: (reviewed: boolean) => void;
  onRetryHome: () => void;
  onSignOut: () => void;
  onUpdate: (patch: Partial<MobileOnboardingDraft>) => void;
  profile: MobileOnboardingDraft;
  usernameReviewed: boolean;
  usernameUnavailable: boolean;
};

function Field({
  children,
  label,
}: {
  children: ReactNode;
  label: string;
}) {
  return <View style={styles.field}>
    <ThemedText style={styles.label}>{label}</ThemedText>
    {children}
  </View>;
}

function ToggleRow({
  disabled = false,
  label,
  onValueChange,
  value,
}: {
  disabled?: boolean;
  label: string;
  onValueChange: (value: boolean) => void;
  value: boolean;
}) {
  return <View style={[styles.toggleRow, disabled ? styles.disabled : null]}>
    <ThemedText style={styles.toggleLabel}>{label}</ThemedText>
    <Switch
      accessibilityLabel={label}
      accessibilityRole="switch"
      accessibilityState={{ checked: value, disabled }}
      disabled={disabled}
      onValueChange={onValueChange}
      value={value}
    />
  </View>;
}

export function OnboardingForm({
  busy,
  canSubmit,
  errorText,
  homeRecoveryStatus,
  onChangeDisplayName,
  onChangeUsername,
  onConfirm,
  onOpenPolicy,
  onReviewUsername,
  onRetryHome,
  onSignOut,
  onUpdate,
  profile,
  usernameUnavailable,
}: OnboardingFormProps) {
  const { fontScale, width } = useWindowDimensions();
  const stackChoices = needsCompactVerticalLayout(width, fontScale);
  const usernameInput = useRef<ElementRef<typeof ThemedTextInput>>(null);
  const displayNameErrorText = validProfileDisplayName(profile.displayName)
    ? undefined
    : i18n.t('onboarding.displayNameGuidance');
  const usernameErrorText = !validProfileUsername(profile.usernameSuggestion)
    ? i18n.t('onboarding.usernameFormatGuidance')
    : usernameUnavailable
      ? i18n.t('onboarding.usernameUnavailable')
      : undefined;

  const [step, setStep] = useState<OnboardingStep>('identity');
  const policyToken = useRef(profile.policyReviewToken);
  useEffect(() => { if (usernameUnavailable) setStep('identity'); }, [usernameUnavailable]);
  useEffect(() => {
    if (policyToken.current !== profile.policyReviewToken) {
      policyToken.current = profile.policyReviewToken;
      setStep('review');
    }
  }, [profile.policyReviewToken]);
  const validStep = step === 'identity' ? !displayNameErrorText && !usernameErrorText
    : step === 'preferences' ? Boolean(profile.profileVisibility && profile.timeZone && profile.firstDayOfWeek)
    : canSubmit;
  const heading = step === 'identity' ? 'onboarding.identityHeading'
    : step === 'preferences' ? 'onboarding.preferencesHeading' : 'onboarding.reviewHeading';
  const description = step === 'identity' ? 'onboarding.identityBody'
    : step === 'preferences' ? 'onboarding.preferencesBody' : 'onboarding.reviewBody';
  function next() {
    if (busy || !validStep) return;
    if (step === 'review') { onConfirm(); return; }
    if (step === 'identity') onReviewUsername(true);
    setStep(current => advanceOnboardingStep(current, validStep, busy));
  }
  function back() {
    if (step === 'identity') onSignOut();
    else setStep(current => previousOnboardingStep(current, busy));
  }

  return <NativeSheet
    compact
    dismissible={false}
    leadingAction={{
      disabled: busy && !homeRecoveryStatus,
      label: i18n.t(step === 'identity' || homeRecoveryStatus ? 'auth.signOut' : 'onboarding.backAction'),
      onPress: homeRecoveryStatus ? onSignOut : back,
    }}
    onRequestClose={onSignOut}
    title={i18n.t(heading)}
    trailingAction={{
      disabled: busy || !validStep,
      label: i18n.t(step === 'review' ? 'onboarding.completeAction' : 'onboarding.continueAction'),
      onPress: next,
    }}
    visible
  >
    <ThemedText style={styles.supportingText}>{i18n.t('onboarding.stepProgress', { current: onboardingSteps.indexOf(step) + 1, total: onboardingSteps.length })}</ThemedText>
    <ThemedText style={styles.intro}>{i18n.t(description)}</ThemedText>
    {step === 'identity' ? <>

    <SectionHeading>{i18n.t('onboarding.accountSection')}</SectionHeading>
    <Surface>
      <Field label={i18n.t('onboarding.privateEmailLabel')}>
        <ThemedText selectable style={styles.accountValue}>{profile.email}</ThemedText>
      </Field>
      <ThemedText style={styles.supportingText}>{i18n.t('onboarding.privateEmailNotice')}</ThemedText>
    </Surface>

    <SectionHeading>{i18n.t('onboarding.profileSection')}</SectionHeading>
    <Surface>
      <Field label={i18n.t('onboarding.displayNameLabel')}>
        <ThemedTextInput
          accessibilityLabel={i18n.t('onboarding.displayNameLabel')}
          accessibilityHint={displayNameErrorText}
          autoComplete="name"
          editable={!busy}
          onChangeText={onChangeDisplayName}
          onSubmitEditing={() => usernameInput.current?.focus()}
          returnKeyType="next"
          submitBehavior="submit"
          value={profile.displayName}
        />
        {displayNameErrorText ? <ThemedText
          accessibilityLiveRegion="polite"
          accessibilityRole="alert"
          style={styles.fieldError}
        >{displayNameErrorText}</ThemedText> : null}
      </Field>
      <Field label={i18n.t('onboarding.usernameLabel')}>
        <ThemedTextInput
          accessibilityLabel={i18n.t('onboarding.usernameLabel')}
          accessibilityHint={usernameErrorText}
          autoCapitalize="none"
          autoComplete="username"
          autoCorrect={false}
          enterKeyHint="done"
          editable={!busy}
          maxLength={64}
          onChangeText={onChangeUsername}
          ref={usernameInput}
          returnKeyType="done"
          value={profile.usernameSuggestion}
        />
        {usernameErrorText ? <ThemedText
          accessibilityLiveRegion="polite"
          accessibilityRole="alert"
          style={styles.fieldError}
        >{usernameErrorText}</ThemedText> : null}
      </Field>
      <ThemedText style={styles.supportingText}>{i18n.t('onboarding.usernameNotice')}</ThemedText>
    </Surface>
    </> : null}
    {step === 'preferences' ? <>
    <Surface>
      <Field label={i18n.t('onboarding.visibilityLabel')}>
        {!profile.profileVisibility
          ? <ThemedText style={styles.supportingText}>{i18n.t('onboarding.visibilityChoose')}</ThemedText>
          : null}
        <View accessibilityRole="radiogroup" style={[styles.choiceGroup, stackChoices ? styles.stackedChoices : null]}>
          <View style={styles.choice}>
            <ActionButton
              accessibilityLabel={i18n.t('onboarding.visibilityPublic')}
              disabled={busy}
              label={i18n.t('onboarding.visibilityPublic')}
              onPress={() => onUpdate({ profileVisibility: 'public' })}
              selected={profile.profileVisibility === 'public'}
              variant={profile.profileVisibility === 'public' ? 'primary' : 'secondary'}
            />
          </View>
          <View style={styles.choice}>
            <ActionButton
              accessibilityLabel={i18n.t('onboarding.visibilityPrivate')}
              disabled={busy}
              label={i18n.t('onboarding.visibilityPrivate')}
              onPress={() => onUpdate({ profileVisibility: 'private' })}
              selected={profile.profileVisibility === 'private'}
              variant={profile.profileVisibility === 'private' ? 'primary' : 'secondary'}
            />
          </View>
        </View>
      </Field>
    </Surface>

    {!profile.firstDayOfWeek
      ? <StatusBanner text={i18n.t('onboarding.localeUnavailable')} tone="error" />
      : null}
    {!profile.timeZone
      ? <StatusBanner text={i18n.t('onboarding.timeZoneUnavailable')} tone="error" />
      : null}

    </> : null}
    {step === 'review' ? <>
    <SectionHeading>{i18n.t('onboarding.privacySection')}</SectionHeading>
    <Surface>
      <ActionButton
        disabled={busy}
        label={i18n.t('onboarding.termsLink', { version: profile.policies.termsOfService.version })}
        onPress={() => onOpenPolicy(profile.policies.termsOfService.url)}
        variant="secondary"
      />
      <ActionButton
        disabled={busy}
        label={i18n.t('onboarding.privacyLink', { version: profile.policies.privacyPolicy.version })}
        onPress={() => onOpenPolicy(profile.policies.privacyPolicy.url)}
        variant="secondary"
      />
      <ActionButton
        disabled={busy}
        label={i18n.t('onboarding.guidelinesLink', { version: profile.policies.communityGuidelines.version })}
        onPress={() => onOpenPolicy(profile.policies.communityGuidelines.url)}
        variant="secondary"
      />
      <ActionButton
        disabled={busy}
        label={i18n.t('onboarding.supportLink')}
        onPress={() => onOpenPolicy(profile.policies.supportUrl)}
        variant="quiet"
      />
    </Surface>

    <SectionHeading>{i18n.t('onboarding.reviewSection')}</SectionHeading>
    <Surface>
      <ToggleRow
        disabled={busy}
        label={i18n.t('onboarding.ageAttestation')}
        onValueChange={(atLeast16) => onUpdate({ atLeast16 })}
        value={profile.atLeast16}
      />
      <ToggleRow
        disabled={busy}
        label={i18n.t('onboarding.termsAcceptance')}
        onValueChange={(termsAccepted) => onUpdate({ termsAccepted })}
        value={profile.termsAccepted}
      />
      <ToggleRow
        disabled={busy}
        label={i18n.t('onboarding.privacyAcknowledgement')}
        onValueChange={(privacyAcknowledged) => onUpdate({ privacyAcknowledged })}
        value={profile.privacyAcknowledged}
      />
      <ToggleRow
        disabled={busy}
        label={i18n.t('onboarding.guidelinesAcceptance')}
        onValueChange={(communityGuidelinesAccepted) => onUpdate({ communityGuidelinesAccepted })}
        value={profile.communityGuidelinesAccepted}
      />
    </Surface>

    </> : null}
    {!validStep && !busy
      ? <ThemedText accessibilityLiveRegion="polite" style={styles.supportingText}>
          {i18n.t('onboarding.requirementsPending')}
        </ThemedText>
      : null}
    {errorText ? <StatusBanner text={errorText} tone="error" /> : null}
    {homeRecoveryStatus === 'loading'
      ? <StatusBanner text={i18n.t('onboarding.homeLoading')} />
      : homeRecoveryStatus
        ? <StatusBanner
            actionLabel={i18n.t('common.retry')}
            onAction={onRetryHome}
            text={i18n.t(homeRecoveryStatus === 'offline' ? 'onboarding.homeOffline' : 'onboarding.homeError')}
            tone={homeRecoveryStatus === 'offline' ? 'offline' : 'error'}
          />
        : busy ? <StatusBanner text={i18n.t('onboarding.confirming')} /> : null}
  </NativeSheet>;
}

const styles = StyleSheet.create({
  accountValue: {
    color: mobileTheme.colors.text,
    ...mobileTheme.typography.subheading,
  },
  choice: {
    flex: 1,
    minWidth: 140,
  },
  choiceGroup: {
    flexDirection: 'row',
    gap: mobileTheme.spacing.sm,
  },
  disabled: {
    opacity: 0.48,
  },
  field: {
    gap: mobileTheme.spacing.xs,
  },
  fieldError: {
    color: mobileTheme.colors.error,
    ...mobileTheme.typography.caption,
  },
  intro: {
    color: mobileTheme.colors.textMuted,
  },
  label: {
    color: mobileTheme.colors.textMuted,
    ...mobileTheme.typography.caption,
  },
  stackedChoices: {
    flexDirection: 'column',
  },
  supportingText: {
    color: mobileTheme.colors.textMuted,
  },
  toggleLabel: {
    flex: 1,
    paddingRight: mobileTheme.spacing.sm,
  },
  toggleRow: {
    alignItems: 'center',
    flexDirection: 'row',
    minHeight: mobileTheme.sizes.minimumTouchTarget,
  },
});
