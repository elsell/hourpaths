import { AccountExport } from './account-export';
import { PolicyTimers } from './policy-timers';
import { useEffect, useState, type ReactNode } from 'react';
import type { Translator } from '@hourpaths/i18n';
import type { PolicyReviewController } from '@hourpaths/client-core';
import { ActionButton, ScreenHeader, ThemedText } from './primitives';
import { SettingsSection, SettingsShell, SettingsSwitchRow } from './settings-list';

export function PolicyReviewView({ controller, i18n, actions, showHeading = true }: { controller: PolicyReviewController; i18n: Translator; actions?: ReactNode; showHeading?: boolean }) {
  const [state, setState] = useState(controller.state);
  const [confirmed, setConfirmed] = useState({ termsAccepted: false, privacyAcknowledged: false, guidelinesAccepted: false });
  useEffect(() => { setState(controller.state); return controller.subscribe(() => setState(controller.state)); }, [controller]);
  useEffect(() => { setConfirmed({ termsAccepted: false, privacyAcknowledged: false, guidelinesAccepted: false }); }, [state.review?.token]);
  const busy = state.loading || state.saving || ['collecting', 'saving'].includes(controller.exportData?.state.phase ?? '');
  return <SettingsShell>
    {showHeading && <ScreenHeader title={i18n.t('policyReview.heading')} />}
    <ThemedText>{i18n.t('policyReview.body')}</ThemedText>
    {state.review && ([
      ['terms', 'termsAccepted', 'onboarding.termsAcceptance', 'onboarding.termsLink'],
      ['privacy', 'privacyAcknowledged', 'onboarding.privacyAcknowledgement', 'onboarding.privacyLink'],
      ['guidelines', 'guidelinesAccepted', 'onboarding.guidelinesAcceptance', 'onboarding.guidelinesLink'],
    ] as const).map(([kind, field, label, link]) => <SettingsSection key={kind}>
      <SettingsSwitchRow label={i18n.t(label)} accessibilityLabel={i18n.t(label)} value={confirmed[field]} disabled={busy}
        valueLabel={i18n.t(confirmed[field] ? 'policyReview.confirmed' : 'policyReview.notConfirmed')}
        onValueChange={value => setConfirmed(previous => ({ ...previous, [field]: value }))} />
      <ActionButton variant="quiet" label={i18n.t(link, { version: state.review!.policies[kind].version })} onPress={() => void controller.openPolicy(kind)} />
    </SettingsSection>)}
    {state.error && <ThemedText accessibilityRole="alert">{i18n.t(state.error === 'review_changed' ? 'policyReview.changed' : state.error === 'invalid' ? 'policyReview.invalid' : 'policyReview.unavailable')}</ThemedText>}
    {state.loading && <ThemedText accessibilityLiveRegion="polite">{i18n.t('common.loading')}</ThemedText>}
    {state.error && <ActionButton variant="secondary" label={i18n.t('common.retry')} disabled={busy} onPress={() => void controller.refresh()} />}
    <ActionButton label={i18n.t('policyReview.continue')} busy={state.saving} disabled={busy || !state.review || !Object.values(confirmed).every(Boolean)} onPress={() => void controller.accept(confirmed)} />
    {controller.timers && state.review && <PolicyTimers controller={controller.timers} i18n={i18n} />}
    {controller.exportData && state.review && <AccountExport controller={controller.exportData} i18n={i18n} />}
    {actions}
  </SettingsShell>;
}
