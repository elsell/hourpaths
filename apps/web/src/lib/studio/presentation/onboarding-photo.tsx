import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { onboardingSteps } from '@hourpaths/client-core';
import type { StudioDependencies } from './app';
import { PictureSettings } from './picture-settings';

export function OnboardingPhotoPage({ dependencies: d }: { dependencies: StudioDependencies }) {
  const [saved, setSaved] = useState(false);
  const identity = useQuery({ queryKey: [d.accountScope, 'preferences', 'identity'], queryFn: ({ signal }) => d.preferences.identity(signal) });
  return <div className="studio studio-entry"><header className="studio-entry-brand"><span className="studio-brand">HourPaths</span></header>
    <main className="studio-entry-card">
      <p className="studio-entry-hint">{d.i18n.t('onboarding.stepProgress', { current: onboardingSteps.length, total: onboardingSteps.length })}</p>
      <h1>{d.i18n.t('onboarding.photoHeading')}</h1>
      <p className="studio-entry-intro">{d.i18n.t('onboarding.photoBody')}</p>
      {identity.data ? <PictureSettings key={identity.data.id} owner={identity.data.id} dependencies={d} onSaved={() => setSaved(true)} />
        : identity.isError ? <div role="alert"><p>{d.i18n.t('picture.unavailable')}</p><button onClick={() => void identity.refetch()}>{d.i18n.t('common.retry')}</button></div>
          : <p role="status">{d.i18n.t('common.loading')}</p>}
      <div className="studio-entry-actions"><Link to="/" replace>{d.i18n.t(saved ? 'onboarding.continueAction' : 'onboarding.skipPhoto')}</Link></div>
    </main>
  </div>;
}
