import { error } from '@sveltejs/kit';
import { createTranslator } from '@hourpaths/i18n';
import { policyDocument, policyEffectiveDate } from '$lib/policies/documents';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = ({ params, locals }) => {
  const document = policyDocument(params.policy);
  if (!document) error(404);
  const effectiveDate = createTranslator([locals.locale]).date(
    new Date(`${policyEffectiveDate}T12:00:00Z`),
    { dateStyle: 'long',
      timeZone: 'UTC',
    },
  );
  return { document, effectiveDate };
};
