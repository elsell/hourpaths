import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { EntryController } from '../entry/application/entry-controller';
import type { EntryService } from '../entry/ports/entry-service';
import type { EntryError, EntryFields } from '../entry/domain/entry';
import './studio.css';

const errors: Record<EntryError, MessageKey> = { recovery: 'duplicateEmailRecovery.failed', unavailable: 'errors.temporarilyUnavailable', expired: 'errors.sessionExpired', storage: 'errors.localSessionUnreadable', username: 'onboarding.usernameUnavailable', policy: 'onboarding.policySetChanged', timeZone: 'onboarding.timeZoneUnavailable', locale: 'onboarding.localeUnavailable', validation: 'errors.validationFailed', signIn: 'errors.signInFailed', callback: 'errors.callbackFailed', superseded: 'errors.sessionExpired', rejected: 'errors.apiRejected', forbidden: 'errors.forbidden', identity: 'errors.identityTokenRejected', rateLimited: 'errors.rateLimited' };
export function AccountEntry({ service, i18n, callback = false, retained }: { service: EntryService; i18n: Translator; callback?: boolean; retained?: ReactNode }) {
  const [, update] = useState(0);
  const [controller] = useState(() => new EntryController(service, () => Date.now(), () => update(value => value + 1)));
  useEffect(() => { void controller.initialize(callback); const timer = setInterval(() => controller.maintain(), 1000); return () => { clearInterval(timer); controller.dispose(); }; }, [controller, callback]);
  const state = controller.state;
  const title = state.phase === 'onboarding' ? 'onboarding.heading' : state.phase === 'recovery' ? 'duplicateEmailRecovery.heading' : retained ? 'offline.signInRequired' : 'auth.welcomeHeading';
  return <div className="studio studio-entry"><header className="studio-entry-brand"><a className="studio-brand" href="/studio"><span aria-hidden="true">◉</span> HourPaths</a></header><main className="studio-entry-card"><h1>{i18n.t(title)}</h1>
    {state.phase === 'loading' || state.phase === 'home' ? <p role="status">{i18n.t(callback ? 'auth.signingIn' : 'common.loading')}</p> : null}
    {state.phase === 'entry' && <>{retained ?? <p className="studio-entry-intro">{i18n.t('auth.welcomeBody')}</p>}<button className="studio-primary studio-entry-primary" disabled={state.busy} onClick={() => void controller.begin()}>{i18n.t('auth.signIn')}</button>{state.busy && <p role="status">{i18n.t('auth.preparingSignIn')}</p>}</>}
    {state.phase === 'onboarding' && <Onboarding controller={controller} i18n={i18n} />}
    {state.phase === 'recovery' && <><p>{i18n.t('duplicateEmailRecovery.notice')}</p><p>{i18n.t('duplicateEmailRecovery.recoverExplanation')}</p><button className="studio-primary studio-entry-primary" disabled={state.busy} onClick={() => void controller.recover()}>{i18n.t('duplicateEmailRecovery.recover')}</button><p className="studio-entry-intro">{i18n.t('duplicateEmailRecovery.separateExplanation')}</p><div className="studio-entry-actions"><button disabled={state.busy} onClick={() => void controller.signOut()}>{i18n.t('duplicateEmailRecovery.returnToSignIn')}</button><button disabled={state.busy} onClick={() => void controller.decline()}>{i18n.t('duplicateEmailRecovery.decline')}</button></div>{state.busy && <p role="status">{i18n.t('common.loading')}</p>}</>}
    {state.error && <div role="alert" className="studio-entry-error" id="entry-error"><p>{i18n.t(errors[state.error])}</p>{state.phase === 'onboarding' && !state.review && <button disabled={state.busy} onClick={() => void controller.retry()}>{i18n.t('common.retry')}</button>}</div>}
  </main></div>;
}
function Onboarding({ controller, i18n }: { controller: EntryController; i18n: Translator }) {
  const state = controller.state, review = state.review, seeded = useRef(false);
  const [fields, setFields] = useState<EntryFields>({ username: '', displayName: '', visibility: '', timeZone: '', firstDayOfWeek: 1, age: false, terms: false, privacy: false, guidelines: false });
  const [reviewToken, setReviewToken] = useState('');
  useEffect(() => {
    if (!review) { setReviewToken(''); setFields(value => ({ ...value, terms: false, privacy: false, guidelines: false })); return; }
    setReviewToken(review.token);
    const shouldSeed = !seeded.current;
    setFields(value => ({ ...value, ...(shouldSeed ? { username: review.username, displayName: review.displayName, ...state.defaults } : {}), terms: false, privacy: false, guidelines: false }));
    seeded.current = true;
  }, [review, state.defaults]);
  function edit<K extends keyof EntryFields>(key: K, value: EntryFields[K]) { setFields(previous => ({ ...previous, [key]: value })); if (state.error === 'username' || state.error === 'validation') controller.clearError(); }
  const weekdays = [1, 2, 3, 4, 5, 6, 7].map(value => ({ value, label: i18n.date(new Date(Date.UTC(2024, 0, value)), {
    weekday: 'long', timeZone: 'UTC',
  }) }));
  function policyLink(kind: 'terms' | 'privacy' | 'guidelines', key: MessageKey) {
    const policy = review?.policies[kind];
    return policy && <button className="studio-entry-link" type="button" onClick={() => controller.openPolicy(policy.url)}>{i18n.t(key, { version: policy.version })}</button>;
  }
  return <><p className="studio-entry-intro">{i18n.t('onboarding.intro')}</p><form className="studio-entry-form" onSubmit={event => { event.preventDefault(); void controller.activate(fields, reviewToken); }}>
    <fieldset disabled={state.busy || !review}>
      <legend>{i18n.t('onboarding.accountSection')}</legend><label>{i18n.t('onboarding.privateEmailLabel')}<input type="email" value={review?.email ?? ''} readOnly /></label><p className="studio-entry-hint">{i18n.t('onboarding.privateEmailNotice')}</p>
      <label>{i18n.t('onboarding.displayNameLabel')}<input value={fields.displayName} autoComplete="name" required onChange={event => edit('displayName', event.target.value)} /></label>
      <label>{i18n.t('onboarding.usernameLabel')}<input value={fields.username} autoComplete="username" autoCapitalize="none" spellCheck={false} minLength={3} maxLength={64} pattern="[A-Za-z0-9_.]+" required aria-invalid={state.error === 'username'} aria-describedby={['entry-username-hint', state.error === 'username' ? 'entry-error' : null].filter(Boolean).join(' ')} onChange={event => edit('username', event.target.value)} /></label><p id="entry-username-hint" className="studio-entry-hint">{i18n.t('onboarding.usernameNotice')}</p>
      <div className="studio-entry-columns"><label>{i18n.t('onboarding.visibilityLabel')}<select value={fields.visibility} required onChange={event => edit('visibility', event.target.value as EntryFields['visibility'])}><option value="" disabled>{i18n.t('onboarding.visibilityChoose')}</option><option value="public">{i18n.t('onboarding.visibilityPublic')}</option><option value="private">{i18n.t('onboarding.visibilityPrivate')}</option></select></label>
      <label>{i18n.t('onboarding.weekStartLabel')}<select value={fields.firstDayOfWeek} onChange={event => edit('firstDayOfWeek', Number(event.target.value))}>{weekdays.map(day => <option key={day.value} value={day.value}>{day.label}</option>)}</select></label></div>
      <p className="studio-entry-hint">{i18n.t('onboarding.timeZoneLabel')}: {fields.timeZone}</p>
    </fieldset>
    <fieldset disabled={state.busy || !review} className="studio-entry-consents"><legend>{i18n.t('onboarding.privacySection')}</legend>
      <label><input type="checkbox" checked={fields.age} required onChange={event => edit('age', event.target.checked)} />{i18n.t('onboarding.ageAttestation')}</label>
      <div><label><input type="checkbox" checked={fields.terms} required onChange={event => edit('terms', event.target.checked)} />{i18n.t('onboarding.termsAcceptance')}</label>{policyLink('terms', 'onboarding.termsLink')}</div>
      <div><label><input type="checkbox" checked={fields.privacy} required onChange={event => edit('privacy', event.target.checked)} />{i18n.t('onboarding.privacyAcknowledgement')}</label>{policyLink('privacy', 'onboarding.privacyLink')}</div>
      <div><label><input type="checkbox" checked={fields.guidelines} required onChange={event => edit('guidelines', event.target.checked)} />{i18n.t('onboarding.guidelinesAcceptance')}</label>{policyLink('guidelines', 'onboarding.guidelinesLink')}</div>
    </fieldset>
    <div className="studio-entry-actions"><button type="button" onClick={() => void controller.signOut()}>{i18n.t('auth.signOut')}</button><button className="studio-primary" disabled={state.busy || !review}>{i18n.t('onboarding.confirm')}</button></div>
    {state.busy && <p role="status">{i18n.t(review ? 'onboarding.confirming' : 'common.loading')}</p>}
    {review && <button type="button" className="studio-entry-link" onClick={() => controller.openPolicy(review.policies.support)}>{i18n.t('onboarding.supportLink')}</button>}
  </form></>;
}
