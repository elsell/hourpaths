import type { MessageKey } from '@hourpaths/i18n';

export type PolicyDocument = {
  title: MessageKey;
  sections: ReadonlyArray<{ title: MessageKey; body: MessageKey }>;
};
export const policyVersion = 'public-2026-10-17';
export const policyEffectiveDate = '2026-10-17';

function document(name: 'terms' | 'privacy' | 'guidelines', sections: readonly string[]): PolicyDocument {
  return {
    title: `publicPolicy.${name}.title` as MessageKey,
    sections: sections.map(section => ({
      title: `publicPolicy.${name}.${section}.title` as MessageKey,
      body: `publicPolicy.${name}.${section}.body` as MessageKey,
    })),
  };
}
const documents: Readonly<Record<string, PolicyDocument>> = {
  'legal/terms': document('terms', ['service', 'account', 'content', 'conduct', 'changes']),
  'legal/privacy': document('privacy', ['data', 'use', 'visibility', 'providers', 'controls', 'retention']),
  'community-guidelines': document('guidelines', ['respect', 'safety', 'authenticity', 'response']),
};
export function policyDocument(path: string): PolicyDocument | undefined {
  return Object.hasOwn(documents, path) ? documents[path] : undefined;
}
